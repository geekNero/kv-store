package utility

import (
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
