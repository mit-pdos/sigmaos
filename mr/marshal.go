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
	l2 := int64(len(value))
	if err := binary.Write(wr, binary.LittleEndian, l1); err != nil {
		return fmt.Errorf("%v: mapper write err %v r %v", proc.GetName(), r, err)
	}
	if err := binary.Write(wr, binary.LittleEndian, l2); err != nil {
		return fmt.Errorf("%v: mapper write err %v r %v", proc.GetName(), r, err)
	}
	if n, err := wr.Write([]byte(key)); err != nil || n != len(key) {
		return fmt.Errorf("%v: mapper write err %v r %v", proc.GetName(), r, err)
	}
	if n, err := wr.Write([]byte(value)); err != nil || n != len(value) {
		return fmt.Errorf("%v: mapper write err %v r %v", proc.GetName(), r, err)
	}
	if n, err := wr.Write([]byte(jsonFiller)); err != nil || n != len(jsonFiller) {
		return fmt.Errorf("%v: mapper write err %v r %v", proc.GetName(), r, err)
	}
	return nil
}

func decodeKV(rd io.Reader, v interface{}) error {
	v2 := v.(*interface{})
	kv := (*v2).(*KeyValue)

	var l1 int64
	var l2 int64

	if err := binary.Read(rd, binary.LittleEndian, &l1); err != nil {
		return err
	}

	if err := binary.Read(rd, binary.LittleEndian, &l2); err != nil {
		return err
	}

	b1 := make([]byte, l1)
	b2 := make([]byte, l2)
	b3 := make([]byte, len(jsonFiller))

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
	n, err = io.ReadFull(rd, b3)
	if err != nil {
		return err
	}
	if n != len(jsonFiller) {
		return fmt.Errorf("bad string")
	}
	kv.Key = string(b1)
	kv.Value = string(b2)
	return nil
}
