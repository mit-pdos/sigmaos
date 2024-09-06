package wc

//
// a word-count application for MapReduce.
//

import (
	"bufio"
	"io"
	"strconv"

	"sigmaos/mr"
)

func Map(filename string, rdr io.Reader, split bufio.SplitFunc, emit mr.EmitT) error {
	scanner := bufio.NewScanner(rdr)
	scanner.Split(split)
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
