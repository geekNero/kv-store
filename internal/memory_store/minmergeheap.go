package memorystore

import (
	"container/heap"
	"fmt"
	"log"
	"os"

	"kv_store/internal/spec"
	"kv_store/internal/utility"
)

type HeapEntry struct {
	*spec.SSTEntry
	sstID int
	index int
	level spec.SSTLevel
}

type MinMergeHeap []*HeapEntry

func (h MinMergeHeap) Len() int {
	return len(h)
}

func (h *MinMergeHeap) Push(x any) {
	*h = append(*h, x.(*HeapEntry))
}

func (h *MinMergeHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return item
}

func (h MinMergeHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h MinMergeHeap) Less(i, j int) bool {
	if h[i].Key != h[j].Key {
		return h[i].Key < h[j].Key
	}

	if h[i].level != h[j].level {
		return h[i].level < h[j].level
	}

	return h[i].sstID > h[j].sstID
}

func (h MinMergeHeap) Peek() *HeapEntry {
	if h.Len() == 0 {
		return nil
	}
	return h[0]
}

func compact(iterables []*fileIterator, targetLevel spec.SSTLevel) error {
	// create a min h and push one entry of each sst into the min h.
	// each sst should always have atleast one entry, and if it does not then something went
	// wrong with the sst.
	h := MinMergeHeap{}

	for index, iterable := range iterables {

		err := iterable.open()
		if err != nil {
			log.Println("error when attempting to open iterator, error: ", err.Error())
			return fmt.Errorf("unable to open iterator, error: ", err.Error())
		}

		entry := iterables[index].nextItem()
		if entry == nil {
			log.Println("sst has 0 entries, sstname: ", iterable.fileName)
			iterables[index].close()
			continue
		}
		sstID := utility.ExtractSSTFileNumber(iterable.fileName)
		h.Push(&HeapEntry{
			SSTEntry: entry,
			sstID:    sstID,
			index:    index,
			level:    iterable.level,
		})
	}
	heap.Init(&h)

	// prepare datastructures to hold sst and manifest data

	compactedData := make([]*spec.SSTEntry, 0)

	newSSTs := []*spec.SSTMetaData{}

	// checks if the file iterator has another entry to provide, if yes
	// then adds it to the heap, or closes it if not already closed.
	addNextItemtoHeap := func(index int) {
		if iterables[index].decodeState == decoding {
			entry := iterables[index].nextItem()
			if entry == nil {
				err := iterables[index].close()
				if err != nil {
					log.Println("failed to close exhausted sst file: ", iterables[index].fileName)
				}
				return
			}

			heap.Push(&h, &HeapEntry{
				SSTEntry: entry,
				sstID:    utility.ExtractSSTFileNumber(iterables[index].fileName),
				index:    index,
				level:    iterables[index].level,
			})
		}
	}

	for h.Len() > 0 {
		top := heap.Pop(&h).(*HeapEntry)
		// Drop tombstone only on the last level
		if !top.Tombstone && targetLevel < spec.MaxLevel {
			compactedData = append(compactedData, top.SSTEntry)
		}

		// move the iterator forward
		addNextItemtoHeap(top.index)

		// flush into an sst file if data has more than MemTableSize entries.
		// TODO: stream this data to the new SST file instead of writing it all at once.
		// To stream, I would have to write the square brackets, and commas on my own, without the help of json package.
		if len(compactedData) >= utility.MemTableSize {
			sstInfo, err := writeSST(compactedData, targetLevel)
			if err != nil {
				log.Println("failed to write compacted data to sst file, error: ", err.Error())
				return err
			}
			newSSTs = append(newSSTs, sstInfo)
			compactedData = make([]*spec.SSTEntry, 0)
		}

		// flush out the older versions of sst keys
		for h.Len() > 0 && h.Peek().Key == top.Key {
			disposableEntry := heap.Pop(&h).(*HeapEntry)
			addNextItemtoHeap(disposableEntry.index)
		}
	}

	if len(compactedData) > 0 {
		sstInfo, err := writeSST(compactedData, targetLevel)
		if err != nil {
			log.Println("failed to write compacted data to sst file, error: ", err.Error())
			return err
		}
		newSSTs = append(newSSTs, sstInfo)
	}

	levelManifest := manifest[targetLevel]
	finalLevelManifest := []*spec.SSTMetaData{}

	deletedIndex := 0
	newIndex := 0
	index := 0

	// delete older SST entries and add new SST entries while maintaining the sorted order of the manifest.
	// deletedIndex might not reach it's end as deleted SSTs could be from a different level
	for index < len(levelManifest) || newIndex < len(newSSTs) {
		if deletedIndex < len(iterables) && iterables[deletedIndex].fileName == levelManifest[index].Name {
			deletedIndex++
			index++
			continue
		}

		if index < len(levelManifest) && newIndex < len(newSSTs) {
			if levelManifest[index].FirstKey < newSSTs[newIndex].FirstKey {
				finalLevelManifest = append(finalLevelManifest, levelManifest[index])
				index++
			} else {
				finalLevelManifest = append(finalLevelManifest, newSSTs[newIndex])
				newIndex++
			}
		} else if index < len(levelManifest) {
			finalLevelManifest = append(finalLevelManifest, levelManifest[index])
			index++
		} else if newIndex < len(newSSTs) {
			finalLevelManifest = append(finalLevelManifest, newSSTs[newIndex])
			newIndex++
		} else {
			fmt.Println("something has gone wrong, else condition should have never been hit!")
		}

	}

	manifest[targetLevel] = finalLevelManifest

	err := flushManifest()
	if err != nil {
		log.Println("failed to flush manifest after compaction, error: ", err.Error())
		return err
	}

	// delete older ssts after the new ssts have been written to.
	for _, file := range iterables {
		err := os.Remove(file.fileName)
		if err != nil {
			log.Printf("failed to purge older sst: %s after compaction, error: %s", file, err.Error())
		}
	}

	return nil
}
