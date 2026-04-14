package memorystore

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"kv_store/internal/config"
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
	level    spec.SSTLevel
	index    int
	decodeState
}

func NewFileIterator(index int, level spec.SSTLevel) *fileIterator {
	fileName := manifest[level][index].Name
	path := level.GetSSTPath(fileName)
	f, err := os.Open(path)
	if err != nil {
		log.Printf("failed to open file %s during compaction, error: %s", path, err.Error())
		return nil
	}
	iterator := &fileIterator{
		f:           f,
		decoder:     json.NewDecoder(f),
		decodeState: start,
		fileName:    fileName,
		level:       level,
		index:       index,
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

	if bracket != json.Delim('[') {
		log.Println("first token isn't a square bracket, token found: ", bracket)
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
		entry := spec.SSTEntry{}
		err := iterator.decoder.Decode(&entry)
		if err != nil {
			log.Println("failed to decode next item, error: ", err.Error())
			return nil
		}
		return &entry
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

	if bracket != json.Delim(']') {
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
L0 compaction is a special case where all SST files present would be compacted for the first time.
In shift compaction method, L0 files would be compacted with L1 files (around 1000 files). To avoid opening
a ton of file iterators, we will first compact L0 on it's own, after which we can compact it with L1 using only a
few file iterators at a time.
*/
func triggerL0Compaction() error {
	// create iterators for all files in manfiest
	l0 := manifest[spec.SSTLevel(0)]
	iterables := make([]*fileIterator, 0, len(l0))

	for index := range l0 {
		iterator := NewFileIterator(index, spec.SSTLevel(0))
		if iterator == nil {
			continue
		}
		iterables = append(iterables, iterator)
	}

	err := compact(iterables, spec.SSTLevel(0))
	if err != nil {
		log.Println("error occured while compacting l0 into sorted ssts, error: ", err.Error())
		return err
	}

	err = multiLevelCompaction(spec.SSTLevel(0), spec.SSTLevel(1))
	if err != nil {
		log.Println("failed to compact sst-0 to sst-1, error: ", err.Error())
		return err
	}

	resetNegativeCache()

	return nil
}

func multiLevelCompaction(lowerLevel spec.SSTLevel, upperLevel spec.SSTLevel) error {
	if lowerLevel > spec.MaxLevel || upperLevel > spec.MaxLevel {
		return fmt.Errorf("level should be less than max level: %d, lowerLevel: %d, upperLevel: %d", int(spec.MaxLevel), int(lowerLevel), int(upperLevel))
	}

	iterables := []*fileIterator{}

	for i := range manifest[lowerLevel] {
		iterable := NewFileIterator(i, lowerLevel)
		if iterable == nil {
			continue
		}

		iterables = append(iterables, iterable)

		if len(iterables) == int(config.Conf.CompactionBatchSize) {
			iterables = addOverlappingSSTRange(iterables, upperLevel)
		}

		err := compact(iterables, upperLevel)
		if err != nil {
			fmt.Printf("failed to compact level %d into level %d, error: %s", int(lowerLevel), int(upperLevel), err.Error())
			return err
		}
	}

	// empty the lower level
	lowerManifest := manifest[lowerLevel]
	manifest[lowerLevel] = make([]*spec.SSTMetaData, 0)

	for _, file := range lowerManifest {
		os.Remove(lowerLevel.GetSSTPath(file.Name))
	}

	return nil
}

func addOverlappingSSTRange(iterables []*fileIterator, level spec.SSTLevel) []*fileIterator {
	lastIndex := len(iterables) - 1
	sourceManifest := manifest[iterables[0].level]

	firstKey := sourceManifest[iterables[0].index].FirstKey
	lastKey := sourceManifest[iterables[lastIndex].index].LastKey

	levelManifest := manifest[level]

	start, end := utility.FindSSTRange(firstKey, lastKey, levelManifest)
	for ; start < end; start++ {
		iterator := NewFileIterator(start, level)
		if iterator != nil {
			iterables = append(iterables, iterator)
		}
	}

	return iterables
}
