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
	decoder *json.Decoder
	f       *os.File
	decodeState
}

func NewFileIterator(f *os.File) *fileIterator {
	iterator := &fileIterator{
		f:           f,
		decoder:     json.NewDecoder(f),
		decodeState: start,
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
		log.Println("iterator state is not equal to finish")
		return nil
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

	for index, file := range manifest {
		f, err := os.Open(file)
		if err != nil {
			log.Printf("failed to open file %s during compaction, error: %s", file, err.Error())
			return err
		}
		iterables[index] = NewFileIterator(f)
	}

	return nil
}
