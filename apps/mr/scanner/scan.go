package scanner

import (
	"unicode"
	"unicode/utf8"

	"sigmaos/util/perf"
)

type ScanByteCounter struct {
	bytesRead int
	p         *perf.Perf
}

func NewScanByteCounter(p *perf.Perf) *ScanByteCounter {
	return &ScanByteCounter{0, p}
}

func (sbc *ScanByteCounter) ScanWords(data []byte, atEOF bool) (advance int, token []byte, err error) {
	a, t, e := scanWords(data, atEOF)
	sbc.bytesRead += a
	sbc.p.TptTick(float64(a))
	return a, t, e
}

func (sbc *ScanByteCounter) BytesRead() int {
	return sbc.bytesRead
}

// ScanSeperator returns the index just after the first separator in data.
// If data contains no separator, the separator is the line terminator that
// bufio.Scanner stripped from the token, so the index just after it is
// len(data) (i.e., the whole token is "before the separator").
func ScanSeperator(data []byte) (int, error) {
	start := 0
	for width := 0; start < len(data); start += width {
		var r rune
		r, width = utf8.DecodeRune(data[start:])
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_' {
			return start + width, nil // index of first char after separator
		}
	}
	return len(data), nil
}

// Scan for words for mappers. Implement grep's definition of a word
func scanWords(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Skip leading non letters
	start := 0
	for width := 0; start < len(data); start += width {
		var r rune
		r, width = utf8.DecodeRune(data[start:])
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
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
