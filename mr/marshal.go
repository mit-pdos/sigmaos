package mr

import (
	"encoding/binary"
	"fmt"
	"io"

	"ulambda/proc"
)

const (
	jsonFiller = "AAAAAAAAAAAAA"
)

func encodeKV(wr io.Writer, key, value string, r int) error {
	// Custom JSON marshalling.
	l1 := int64(len(key))
	l2 := int64(len(key))
	if err := binary.Write(wr, binary.LittleEndian, l1); err != nil {
		return fmt.Errorf("%v: mapper write err %v", proc.GetName(), r, err)
	}
	if err := binary.Write(wr, binary.LittleEndian, []byte(key)); err != nil {
		return fmt.Errorf("%v: mapper write err %v", proc.GetName(), r, err)
	}
	if err := binary.Write(wr, binary.LittleEndian, l2); err != nil {
		return fmt.Errorf("%v: mapper write err %v", proc.GetName(), r, err)
	}
	if err := binary.Write(wr, binary.LittleEndian, []byte(value)); err != nil {
		return fmt.Errorf("%v: mapper write err %v", proc.GetName(), r, err)
	}
	return nil
}

func decodeKV(rd io.Reader, v interface{}) error {
	kv := v.(*KeyValue)

	var l1 int64
	var l2 int64

	b1 := make([]byte, l1)
	b2 := make([]byte, l2)

	n, err := io.ReadFull(rd, b1)
	if err != nil {
		return err
	}
	if n != int(l1) {
		return fmt.Errorf("bad string")
	}

	n, err = io.ReadFull(rd, b2)
	if err != nil {
		return err
	}
	if n != int(l2) {
		return fmt.Errorf("bad string")
	}
	kv.Key = string(b1)
	kv.Value = string(b2)
	return nil
}
