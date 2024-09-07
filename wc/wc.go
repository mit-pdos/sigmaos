package wc

//
// a word-count application for MapReduce.
//

import (
	"bufio"
	"strconv"

	"sigmaos/mr"
)

func Map(filename string, scanner *bufio.Scanner, emit mr.EmitT) error {
	kv := &mr.KeyValue{}
	for scanner.Scan() {
		kv.Key = scanner.Text()
		kv.Value = "1"
		if err := emit(kv); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func Reduce(key string, values []string, emit mr.EmitT) error {
	// return the number of occurrences of this word.
	n := 0
	for _, v := range values {
		m, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		n += m
	}
	kv := &mr.KeyValue{key, strconv.Itoa(n)}
	if err := emit(kv); err != nil {
		return err
	}
	return nil
}
