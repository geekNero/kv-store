package utility

import (
	"encoding/json"
	"hash/crc32"
	"regexp"
	"strconv"
	"strings"
	"unicode"
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
