package memorystore

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"

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
	buffer   *spec.SSTEntry
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
		buffer:      &spec.SSTEntry{},
	}
	return iterator
}

func (iterator *fileIterator) open() error {
	// we won't change the state until atleast one item from this iterator is used.
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

	// pre-load atleast one item in the buffer, it's valid to error out if we can't get
	// the first item.
	err = iterator.decoder.Decode(iterator.buffer)
	if err != nil {
		log.Println("failed to buffer first entry while opening iterator, error: ", err.Error())
		return err
	}
	return nil
}

func (iterator *fileIterator) nextItem() *spec.SSTEntry {
	// peek at the iterator value
	return iterator.buffer
}

func (iterator *fileIterator) pop() *spec.SSTEntry {

	// set the decodeState to decoding only when an entry has been popped.
	// nextItem allows one entry accessible even in decodeState = start, refer updateIterables
	if iterator.decodeState == finish {
		return nil
	}
	iterator.decodeState = decoding
	returnItem := iterator.buffer
	if iterator.decoder.More() {
		entry := spec.SSTEntry{}
		err := iterator.decoder.Decode(&entry)
		if err != nil {
			log.Println("failed to decode next item, error: ", err.Error())
			iterator.buffer = nil
			return returnItem
		}
		// return buffer and buffer next item
		iterator.buffer = &entry
		return returnItem
	}

	if iterator.buffer != nil {
		iterator.buffer = nil
		iterator.close()
	}
	return returnItem
}

func (iterator *fileIterator) close() error {

	iterator.decodeState = finish
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

	log.Println("finished first compaction state")

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

	indexUpper, endUpper := utility.FindSSTRange(firstKey, lastKey, upperLevelManifest)
	finalManifest := []*spec.SSTMetaData{}

	lowerIterable := NewFileIterator(0, lowerLevel)
	upperIterable := NewFileIterator(indexUpper, upperLevel)
	deletedSSTs := map[int]struct{}{}

	// upperIterable can be nil if there is no over lap between both the levels or if upperLevel is empty
	if upperIterable != nil {

		// valid crash out if any iterable does not open
		err := lowerIterable.open()
		if err != nil {
			return err
		}
		err = upperIterable.open()
		if err != nil {
			return err
		}

		// set the backing array size so that it does not get reallocated.
		compacteData := make([]*spec.SSTEntry, 0, utility.MemTableSize)

		// the range is centered arround lower level, therefore we can reliably iterate over upper level.
		for upperIterable.index < endUpper {

			// update the compacted data
			op := compareNextIteratorItem(lowerIterable, upperIterable)
			switch op {
			case -1:
				compacteData = pushToCompactedData(compacteData, lowerIterable.pop(), lowerLevel, &finalManifest)
			case 1:
				compacteData = pushToCompactedData(compacteData, upperIterable.pop(), upperLevel, &finalManifest)
				// mark an upperLevel sst as deleted only if it has been popped.

				// if upper level sst is mutated than mark it deleted
				deletedSSTs[upperIterable.index] = struct{}{}
			case 0:
				// if both keys are equal, pick the item from lowerLevel.
				compacteData = pushToCompactedData(compacteData, lowerIterable.pop(), lowerLevel, &finalManifest)
				// drain upperLevel item
				upperIterable.pop()
				// if upper level sst is mutated than mark it deleted
				deletedSSTs[upperIterable.index] = struct{}{}
			}

			// update the iterables
			lowerIterable, upperIterable, err = updateIterables(lowerIterable, upperIterable, lowerLevelManifest, upperLevelManifest, &finalManifest)
			// an error is returned when we do not get an iterable necessary for complete compaction.
			if err != nil {
				log.Println("error when updating iterables during compaction, error: ", err.Error())
				return err
			}

			// edit: after the above code changes, this condition should not be hit in a clean scenario, as the iterables if nil would have an
			// error along with them.
			if lowerIterable == nil || upperIterable == nil {
				break
			}

		}

		// flush the remaining compacted data if any
		if len(compacteData) > 0 {
			metadata, err := writeSST(compacteData, upperLevel)
			if err != nil {
				log.Println("failed to write new sst during compaction, error: ", err.Error())
				return err
			}
			finalManifest = append(finalManifest, metadata)
		}
		// remove deleted ssts and add remaining ssts to final manifest
		for index, sst := range upperLevelManifest {
			if _, ok := deletedSSTs[index]; ok {
				err := os.Remove(upperLevel.GetSSTPath(sst.Name))
				if err != nil {
					log.Printf("failed to delete sst file post compaction, file: %s, error: %v\n", upperLevel.GetSSTPath(sst.Name), err)
				}
			} else {
				finalManifest = append(finalManifest, sst)
			}
		}
	}

	index := len(lowerLevelManifest)
	if lowerIterable != nil {
		index = lowerIterable.index
		if lowerIterable.decodeState == finish {
			index += 1
		}
	}
	// if there are ssts remaining from the lowerLevel, move them to the upper level
	for index < len(lowerLevelManifest) {

		newName := getNextSSTName(upperLevel)
		err := os.Link(lowerLevel.GetSSTPath(lowerLevelManifest[index].Name), upperLevel.GetSSTPath(newName))
		if err != nil {
			log.Printf("failed to create symlink of lower level sst: %s, error: %v\n", lowerLevel.GetSSTPath(lowerLevelManifest[index].Name), err)
			return err
		}

		finalManifest = append(finalManifest, &spec.SSTMetaData{
			Name:     newName,
			FirstKey: lowerLevelManifest[index].FirstKey,
			LastKey:  lowerLevelManifest[index].LastKey,
		})
		index += 1
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

func compareNextIteratorItem(lowerIter *fileIterator, upperIter *fileIterator) int {

	// this allows us to drain the remaining iterator if one iterator is closed
	if upperIter.decodeState == finish {
		return -1
	}

	if lowerIter.decodeState == finish {
		return 1
	}

	if lowerIter.nextItem().Key < upperIter.nextItem().Key {
		return -1
	} else if lowerIter.nextItem().Key > upperIter.nextItem().Key {
		return 1
	}
	return 0
}

// pushToCompactedData handles appending the sst entry to compactedData list, and also handles flushing it to an sst incase it crosses the size limit.
func pushToCompactedData(compactedData []*spec.SSTEntry, item *spec.SSTEntry, level spec.SSTLevel, newSSTs *[]*spec.SSTMetaData) []*spec.SSTEntry {
	compactedData = append(compactedData, item)

	if len(compactedData) == utility.MemTableSize {
		metadata, err := writeSST(compactedData, level)
		if err != nil {
			log.Println("failed to write new sst during compaction, error: ", err.Error())
			return nil
		}

		*newSSTs = append(*newSSTs, metadata)
		return make([]*spec.SSTEntry, 0, utility.MemTableSize)
	}
	return compactedData
}

// add a new iterator for each level if possible, if it errors out break the compaction process.
// if no new iterator is possible for a level, maintain the old iterator to allow the remaining iterator to be drained out.
func updateIterables(lowerIter *fileIterator, upperIter *fileIterator, lowerLevelManifest, upperLevelManifest []*spec.SSTMetaData, newSSTs *[]*spec.SSTMetaData) (*fileIterator, *fileIterator, error) {

	for {
		if lowerIter.decodeState == finish {

			temp := NewFileIterator(lowerIter.index+1, lowerIter.level)
			// for a non-error scenario, do not assign the nil iterator back to retain the state of the old iterator
			if temp != nil {
				err := temp.open()
				if err != nil {
					return nil, upperIter, err
				}
				lowerIter = temp
			}
		}

		if upperIter.decodeState == finish {
			// for a non-error scenario, do not assign the nil iterator back to retain the state of the old iterator
			temp := NewFileIterator(upperIter.index+1, upperIter.level)
			if temp != nil {
				err := temp.open()
				if err != nil {
					return lowerIter, nil, err
				}
				upperIter = temp
			}

		}

		// if both the iterators are new, we can check if they are relevant to each other
		// before using any of them. If level A has an sst with both it's first and last keys lexiographically lower than
		// the other level B, then there are no more ssts to be compared from level B for that SST.
		if lowerIter.decodeState == start && upperIter.decodeState == start {
			op := utility.CompareSSTs(lowerLevelManifest[lowerIter.index], upperLevelManifest[upperIter.index])
			switch op {
			case 1:
				// let the next iteration handle opening of a new file iterator
				upperIter.close()

			case -1:
				// copy the the sst of lower level to the upper level
				newName := getNextSSTName(upperIter.level)
				err := os.Link(lowerIter.level.GetSSTPath(lowerIter.fileName), upperIter.level.GetSSTPath(newName))
				if err != nil {
					log.Printf("failed to create symlink of lower level sst: %s, error: %v\n", lowerIter.level.GetSSTPath(lowerIter.fileName), err)
					return nil, nil, err
				}

				*newSSTs = append(*newSSTs, &spec.SSTMetaData{
					Name:     newName,
					FirstKey: lowerLevelManifest[lowerIter.index].FirstKey,
					LastKey:  lowerLevelManifest[lowerIter.index].LastKey,
				})

				// let the next iteration handle opening of a new file iterator
				lowerIter.close()

			// exit the loop
			default:
				return lowerIter, upperIter, nil
			}
		} else {
			return lowerIter, upperIter, nil
		}
	}
}
