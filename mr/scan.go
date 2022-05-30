package mr

import (
	"unicode"
	"unicode/utf8"
)

// Scan for words for mappers. Implement grep's definition of a word
func ScanWords(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Skip leading non letters
	start := 0
	for width := 0; start < len(data); start += width {
		var r rune
		r, width = utf8.DecodeRune(data[start:])
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r != '_' {
			break
		}
	}
	// Scan until non letter
	for width, i := 0, start; i < len(data); i += width {
		var r rune
		r, width = utf8.DecodeRune(data[i:])
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_' {
			return i + width, data[start:i], nil
		}
	}
	// If we're at EOF, we have a final, non-empty, non-terminated word. Return it.
	if atEOF && len(data) > start {
		return len(data), data[start:], nil
	}
	// Request more data.
	return start, nil, nil

}
