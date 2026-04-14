package memorystore

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"kv_store/internal/spec"
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

	for _, file := range l0 {
		iterator := NewFileIterator(file.Name)
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

	resetNegativeCache()
	mutationCounter = 0

	return nil
}

func multiLevelCompaction(lowerLevel spec.SSTLevel, upperLevel spec.SSTLevel) error {

	if lowerLevel > spec.MaxLevel || upperLevel > spec.MaxLevel {
		return fmt.Errorf("level should be less than max level: %d, lowerLevel: %d, upperLevel: %d", int(spec.MaxLevel), int(lowerLevel), int(upperLevel))
	}

	for _, source := range manifest[lowerLevel] {
		iterables := make([]*fileIterator, 0, 2)
		err := NewFileIterator(source.Name)
	}
}
