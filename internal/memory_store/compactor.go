package memorystore

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"

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
	if index < 0 || index >= len(manifest[level]) {
		return nil
	}

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

func (iterator *fileIterator) close(forceClose bool) error {
	if iterator.decodeState != finish && !forceClose {
		log.Println("iterator state was not equal to finish")
		return fmt.Errorf("iterator state was not equal to finish when closing iterator")
	}

	if !forceClose {

		bracket, err := iterator.decoder.Token()
		if err != nil {
			log.Println("failed to parse file closing bracket, error: ", err.Error())
			return err
		}

		if bracket != json.Delim(']') {
			log.Println("last token isn't a square bracket")
			return fmt.Errorf("last token isn't a square bracket when closing iterator")
		}
	}
	err := iterator.f.Close()
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
	err := compact(spec.SSTLevel(0))
	if err != nil {
		log.Println("error occured while compacting l0 into sorted ssts, error: ", err.Error())
		return err
	}

	err = MergeTheStrips(spec.SSTLevel(0), spec.SSTLevel(1))
	if err != nil {
		log.Println("failed to compact sst-0 to sst-1, error: ", err.Error())
		return err
	}

	resetNegativeCache()

	return nil
}

/*
MergeTheStrips is a simple two way merge where we consider the entire levels
as long sorted list of strings, but broken into smaller strips. All we need to do is keep on merging
the next smallest item from either of the levels, and add it to a new strip. If the item is same between both
the lists we chose the item from the lower level and discard it from the higher level.

Once the strip reaches it's capactiy, we cut it and save it.
*/
func MergeTheStrips(lowerLevel spec.SSTLevel, upperLevel spec.SSTLevel) error {
	if lowerLevel > spec.MaxLevel || upperLevel > spec.MaxLevel {
		return fmt.Errorf("level should be less than max level: %d, lowerLevel: %d, upperLevel: %d", int(spec.MaxLevel), int(lowerLevel), int(upperLevel))
	}

	lowerLevelManifest := manifest[lowerLevel]
	upperLevelManifest := manifest[upperLevel]

	// Narrow the range of upperLevel SSTs to be considered.
	firstKey := lowerLevelManifest[0].FirstKey
	lastKey := lowerLevelManifest[len(lowerLevelManifest)-1].LastKey

	// append new ssts to the upper level and rewrite and sort the level manifest later on.
	// for file in lowerLevel that has to be retained, it should be renamed into the upperLevel and appended
	// to the upperLevel. For the deleted ssts from upperLevel, update the manifest directly, and empty
	// their firstKey.

	getNextItem := func(iterator *fileIterator) *spec.SSTEntry {
		if iterator.decodeState == start {
			err := iterator.open()
			if err != nil {
				return nil
			}
		}

		if iterator.decodeState == decoding {
			entry := iterator.nextItem()
			if entry != nil {
				return entry
			}
			err := iterator.close(false)
			if err != nil {
				log.Println("failed to close iterator in getNextItem, error: ", err.Error())
			}
		}
		return nil
	}

	indexUpper, endUpper := utility.FindSSTRange(firstKey, lastKey, upperLevelManifest)
	indexLower, endLower := 0, len(lowerLevelManifest)

	var itemLower, itemUpper *spec.SSTEntry

	iteratorLowerLevel := NewFileIterator(0, lowerLevel)
	iteratorUpperLevel := NewFileIterator(indexUpper, upperLevel)

	if iteratorLowerLevel == nil || iteratorUpperLevel == nil {
		return fmt.Errorf("lower/upper level iterator is nil")
	}
	compactedData := []*spec.SSTEntry{}

	setNextFileIterator := func(iterator **fileIterator, index *int, level spec.SSTLevel) bool {
		*index++
		*iterator = NewFileIterator(*index, level)
		return *iterator != nil
	}

	flushCompactedData := func() error {
		if len(compactedData) >= utility.MemTableSize {
			sstInfo, err := writeSST(compactedData, upperLevel)
			if err != nil {
				log.Println("failed to write compacted data to sst file, error: ", err.Error())
				return err
			}
			upperLevelManifest = append(upperLevelManifest, sstInfo)
			compactedData = make([]*spec.SSTEntry, 0)
		}
		return nil
	}

	for indexLower < endLower && indexUpper < endUpper {

		if iteratorLowerLevel.decodeState == iteratorUpperLevel.decodeState && iteratorLowerLevel.decodeState == start {
			op := utility.IfSSTsIntersect(lowerLevelManifest[indexLower], upperLevelManifest[indexUpper])
			// if lowerLevelSST has keys greater than the upperLevelSST, then the next upperLevelSST might
			// intersect with the lowerLevelSST, hence we move to the next upper sst. The converse is not
			// true.
			if op == 1 {
				iteratorUpperLevel.close(true)
				if !setNextFileIterator(&iteratorUpperLevel, &indexUpper, upperLevel) {
					break
				}
			} else if op == -1 {
				iteratorLowerLevel.close(true)
				lowerLevelManifest[indexLower].Name = getNextSSTName(upperLevel)
				err := os.Rename(lowerLevel.GetSSTPath(iteratorLowerLevel.fileName), upperLevel.GetSSTPath(lowerLevelManifest[indexLower].Name))
				if err != nil {
					fmt.Printf("failed to rename lower sst to upper sst, sstName: %s, level: %d, error: %v", iteratorLowerLevel.fileName, int(lowerLevel), err)
				}
				upperLevelManifest = append(upperLevelManifest, lowerLevelManifest[indexLower])

				if !setNextFileIterator(&iteratorLowerLevel, &indexLower, lowerLevel) {
					break
				}
			} else {
				upperLevelManifest[indexUpper].FirstKey = ""
			}
		}
		if itemLower == nil {
			itemLower = getNextItem(iteratorLowerLevel)
			if itemLower == nil && !setNextFileIterator(&iteratorLowerLevel, &indexLower, lowerLevel) {
				break
			}
		}
		if itemUpper == nil {
			itemUpper = getNextItem(iteratorUpperLevel)
			if itemUpper == nil && !setNextFileIterator(&iteratorUpperLevel, &indexUpper, upperLevel) {
				break
			}
		}

		if itemLower.Key < itemUpper.Key {
			compactedData = append(compactedData, itemLower)
		} else if itemLower.Key > itemUpper.Key {
			compactedData = append(compactedData, itemUpper)
		} else {
			compactedData = append(compactedData, itemLower)
			itemUpper = getNextItem(iteratorUpperLevel)
		}

		if err := flushCompactedData(); err != nil {
			return err
		}
	}

	finalManifest := []*spec.SSTMetaData{}

	// remove deleted ssts
	for _, sst := range upperLevelManifest {
		if sst.FirstKey == "" {
			err := os.Remove(upperLevel.GetSSTPath(sst.Name))
			if err != nil {
				log.Printf("failed to delete sst file post compaction, file: %s, error: %v\n", upperLevel.GetSSTPath(sst.Name), err)
			}
		} else {
			finalManifest = append(finalManifest, sst)
		}
	}

	// sort final manifest on the first keys.
	sort.Slice(finalManifest, func(i, j int) bool {
		return finalManifest[i].FirstKey < finalManifest[j].FirstKey
	})

	manifest[upperLevel] = finalManifest
	manifest[lowerLevel] = []*spec.SSTMetaData{}
	flushManifest()

	err := os.RemoveAll(lowerLevel.FolderString())
	if err != nil {
		log.Printf("failed to delete lower level folder after compaction, folder: %s, error: %v\n", lowerLevel.FolderString(), err)
	}

	err = os.Mkdir(lowerLevel.FolderString(), 0o755)
	if err != nil {
		log.Printf("failed to re-create lower level folder after compaction, folder: %s, error: %v\n", lowerLevel.FolderString(), err)
	}

	return nil
}

func multiLevelCompaction(lowerLevel spec.SSTLevel, upperLevel spec.SSTLevel) error {
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

		// err := compact(iterables, upperLevel)
		// if err != nil {
		// 	fmt.Printf("failed to compact level %d into level %d, error: %s", int(lowerLevel), int(upperLevel), err.Error())
		// 	return err
		// }
	}

	// empty the lower level
	lowerManifest := manifest[lowerLevel]
	manifest[lowerLevel] = make([]*spec.SSTMetaData, 0)
	flushManifest()

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
