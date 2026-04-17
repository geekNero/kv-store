package utility

import (
	"encoding/json"
	"hash/crc32"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"kv_store/internal/spec"
)

func IsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func ExtractSSTFileNumber(name string) int {
	name = strings.Trim(name, "st-.jon")
	x, err := strconv.Atoi(name)
	if err != nil {
		return -1
	}
	return x
}

func IsSSTFile(name string) bool {
	re := regexp.MustCompile(`^sst-\d+\.json$`)
	return re.MatchString(name)
}

func HashStruct(v any) (uint32, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return 0, err
	}

	return crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli)), nil
}

func CheckPtrStringsEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func FindKeyContainingSST(key string, sstSet []*spec.SSTMetaData) *spec.SSTMetaData {
	low := 0
	high := len(sstSet) - 1

	for low <= high {
		mid := (low + high) / 2
		element := sstSet[mid]

		if element.FirstKey <= key && element.LastKey >= key {
			return element
		} else if element.FirstKey > key {
			high = mid - 1
		} else if element.LastKey < key {
			low = mid + 1
		}
	}

	return nil
}

func FindSSTRange(firstKey string, lastKey string, sstSet []*spec.SSTMetaData) (int, int) {
	if len(sstSet) == 0 {
		return 0, 0
	}

	// Find the first SST that could potentially overlap (LastKey >= firstKey)
	start := sort.Search(len(sstSet), func(i int) bool {
		return sstSet[i].LastKey >= firstKey
	})

	// Find the first SST that starts after our range (FirstKey > lastKey)
	end := sort.Search(len(sstSet), func(i int) bool {
		return sstSet[i].FirstKey > lastKey
	})

	return start, end
}

// IfSSTsIntersect returns 1 if sst1 is greater than sst2, -1 for vice-versa and 0 if they intersect
func IfSSTsIntersect(sst1 *spec.SSTMetaData, sst2 *spec.SSTMetaData) int {
	if sst1.FirstKey > sst2.LastKey && sst1.LastKey > sst2.FirstKey {
		return 1
	} else if sst2.FirstKey > sst1.LastKey && sst2.LastKey > sst1.FirstKey {
		return -1
	}
	return 0
}
