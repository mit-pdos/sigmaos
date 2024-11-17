package wc

//
// a word-count application for MapReduce.
//

import (
	"bufio"
	"strconv"

	"sigmaos/mr/mr"
)

func Map(filename string, scanner *bufio.Scanner, emit mr.EmitT) error {
	for scanner.Scan() {
		if err := emit(scanner.Bytes(), "1"); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

// Emit the number of occurrences of this word.
func Reduce(key string, values []string, emit mr.EmitT) error {
	n := 0
	for _, v := range values {
		m, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		n += m
	}
	if err := emit([]byte(key), strconv.Itoa(n)); err != nil {
		return err
	}
	return nil
}
