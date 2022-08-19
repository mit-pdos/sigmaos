package wc

//
// a word-count application for MapReduce.
//

import (
	"bufio"
	"io"
	"strconv"

	"ulambda/mr"
)

func Map(filename string, rdr io.Reader, emit mr.EmitT) error {
	scanner := bufio.NewScanner(rdr)
	scanner.Split(mr.ScanWords)
	for scanner.Scan() {
		kv := &mr.KeyValue{scanner.Text(), "1"}
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
	kv := &mr.KeyValue{key, strconv.Itoa(len(values))}
	if err := emit(kv); err != nil {
		return err
	}
	return nil
}
