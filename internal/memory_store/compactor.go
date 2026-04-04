package memorystore

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"kv_store/internal/spec"
	"kv_store/internal/utility"
)

type decodeState string

const (
	start    decodeState = "start"
	decoding decodeState = "decoding"
	finish   decodeState = "finish"
)

type fileIterator struct {
	decoder  *json.Decoder
	f        *os.File
	fileName string
	decodeState
}

func NewFileIterator(fileName string) *fileIterator {

	f, err := os.Open(fileName)
	if err != nil {
		log.Printf("failed to open file %s during compaction, error: %s", fileName, err.Error())
		return nil
	}
	iterator := &fileIterator{
		f:           f,
		decoder:     json.NewDecoder(f),
		decodeState: start,
		fileName:    fileName,
	}
	return iterator
}

func (iterator *fileIterator) open() error {
	if iterator.decodeState != start {
		log.Println("iterator state is not equal to start")
		return fmt.Errorf("iterator state not equal to start when opening iterator")
	}

	bracket, err := iterator.decoder.Token()
	if err != nil {
		log.Println("failed to parse file opening bracket, error: ", err.Error())
		return err
	}

	if bracket != "[" {
		log.Println("first token isn't a square bracket")
		return fmt.Errorf("first token isn't a square bracket when opening iterator")
	}

	iterator.decodeState = decoding
	return nil
}

func (iterator *fileIterator) nextItem() *spec.SSTEntry {
	if iterator.decodeState != decoding {
		log.Println("iterator state is not equal to decoding")
		return nil
	}

	if iterator.decoder.More() {
		var entry *spec.SSTEntry
		err := iterator.decoder.Decode(entry)
		if err != nil {
			log.Println("failed to decode next item, error: ", err.Error())
			return nil
		}
		return entry
	}

	// mark it done if no more json items remain for parsing
	iterator.decodeState = finish
	return nil
}

func (iterator *fileIterator) close() error {
	if iterator.decodeState != finish {
		log.Println("iterator state was not equal to finish")
		return fmt.Errorf("iterator state was not equal to finish when closing iterator")
	}

	bracket, err := iterator.decoder.Token()
	if err != nil {
		log.Println("failed to parse file closing bracket, error: ", err.Error())
		return err
	}

	if bracket != "]" {
		log.Println("last token isn't a square bracket")
		return fmt.Errorf("last token isn't a square bracket when closing iterator")
	}

	err = iterator.f.Close()
	if err != nil {
		log.Println("failure while attempting to close file handler, error: ", err.Error())
		return err
	}
	return nil
}

/*
In the compaciton process, we extract the latest representaion of each key value pair and
create new SSTs that only represent those. In doing so, we rewrite the manifest as well.
We could reset the numbering of SSTs or we could continue it, our operations won't be impacted either
way as long as the manifest is consistent.

Before compaction, the higher the SST number is, newer are the entries it has.
After compaction, the higher SSTs simply contain lexiographically decreasing key value pairs.
*/
func triggerCompaction() error {
	// create iterators for all files in manfiest
	iterables := make([]*fileIterator, 0, len(manifest))

	// create a min h and push one entry of each sst into the min h.
	// each sst should always have atleast one entry, and if it does not then something went
	// wrong with the sst.
	h := MinMergeHeap{}
	openSSTs := 0

	for index, file := range manifest {
		iterator := NewFileIterator(file)
		if iterator == nil {
			continue
		}
		iterables = append(iterables, iterator)

		entry := iterables[index].nextItem()
		if entry == nil {
			log.Println("sst has 0 entries, sstname: ", file)
			iterables[index].close()
			continue
		}
		openSSTs++

		sstID := utility.ExtractSSTFileNumber(file)
		h.Push(&HeapEntry{
			SSTEntry: entry,
			sstID:    sstID,
			index:    index,
		})
	}
	heap.Init(&h)

	// prepare datastructures to hold sst and manifest data
	compactedData := make([]*spec.SSTEntry, 0)
	newManifest := make([]string, 0)

	// checks if the file iterator has another entry to provide, if yes
	// then adds it to the heap, or closes it if not already closed.
	addNextItemtoHeap := func(index int) {
		if iterables[index].decodeState == decoding {
			entry := iterables[index].nextItem()
			if entry == nil {
				openSSTs--
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
			})
		}
	}

	for openSSTs > 0 {
		top := heap.Pop(&h).(HeapEntry)
		if !top.Tombstone {
			compactedData = append(compactedData, top.SSTEntry)
		}

		// move the iterator forward
		addNextItemtoHeap(top.index)

		// flush into an sst file if data has more than MemTableSize entries.
		if len(compactedData) >= utility.MemTableSize {
			sstName, err := writeSST(compactedData)
			if err != nil {
				log.Println("failed to write compacted data to sst file, error: ", err.Error())
				return err
			}
			newManifest = append(newManifest, sstName)
			compactedData = make([]*spec.SSTEntry, 0)
		}

		// flush out the older versions of sst keys
		for h.Len() > 0 && h.Peek().Key == top.Key {
			disposableEntry := heap.Pop(&h).(HeapEntry)
			addNextItemtoHeap(disposableEntry.index)
		}
	}

	if len(compactedData) > 0 {
		sstName, err := writeSST(compactedData)
		if err != nil {
			log.Println("failed to write compacted data to sst file, error: ", err.Error())
			return err
		}
		newManifest = append(newManifest, sstName)
	}

	manifest = newManifest
	err := flushManifest()
	if err != nil {
		log.Println("failed to flush manifest after compaction, error: ", err.Error())
		return err
	}

	return nil
}
