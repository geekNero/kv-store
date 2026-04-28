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

func compact(targetLevel spec.SSTLevel) error {
	// create a min h and push one entry of each sst into the min h.
	// each sst should always have atleast one entry, and if it does not then something went
	// wrong with the sst.
	h := MinMergeHeap{}

	l := manifest[targetLevel]
	iterables := make([]*fileIterator, 0, len(l))

	for index := range l {
		iterator := NewFileIterator(index, targetLevel)
		if iterator == nil {
			continue
		}

		err := iterator.open()
		if err != nil {
			log.Println("error when attempting to open iterator, error: ", err.Error())
			return fmt.Errorf("unable to open iterator, error: %s", err.Error())
		}

		iterables = append(iterables, iterator)

		entry := iterator.pop()
		if entry == nil {
			log.Println("sst has 0 entries, sstname: ", iterator.fileName)
			continue
		}
		sstID := utility.ExtractSSTFileNumber(iterator.fileName)
		h.Push(&HeapEntry{
			SSTEntry: entry,
			sstID:    sstID,
			index:    index,
			level:    iterator.level,
		})
	}
	heap.Init(&h)

	// prepare datastructures to hold sst and manifest data

	compactedData := make([]*spec.SSTEntry, 0)

	// checks if the file iterator has another entry to provide, if yes
	// then adds it to the heap, or closes it if not already closed.
	addNextItemtoHeap := func(index int) {
		if iterables[index].decodeState == decoding {
			entry := iterables[index].pop()
			if entry == nil {
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

	newSSTs := []*spec.SSTMetaData{}

	for h.Len() > 0 {
		top := heap.Pop(&h).(*HeapEntry)
		// Drop tombstone only on the last level
		if !(top.Tombstone && targetLevel == spec.DefaultMaxLevel) {
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

	// delete old sst metadata and replace it with new ssts while keeping the manifest sorted
	for _, file := range iterables {

		err := os.Remove(file.level.GetSSTPath(file.fileName))
		if err != nil {
			log.Printf("failed to purge older sst: %s after compaction, error: %s", file.fileName, err.Error())
		}

	}

	manifest[targetLevel] = newSSTs

	err := flushManifest()
	if err != nil {
		log.Println("failed to flush manifest after compaction, error: ", err.Error())
		return err
	}
	return nil
}
