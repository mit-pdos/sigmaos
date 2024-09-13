package mr

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	jsonPadding = "AAAAA"
)

func encodeKV(wr io.Writer, key []byte, value string, r int) (int, error) {
	// Custom JSON marshalling.
	l1 := int64(len(key))
	l2 := int64(len(value))
	if err := binary.Write(wr, binary.LittleEndian, l1); err != nil {
		return 0, fmt.Errorf("mapper write err %v r %v", r, err)
	}
	if err := binary.Write(wr, binary.LittleEndian, l2); err != nil {
		return 0, fmt.Errorf("mapper write err %v r %v", r, err)
	}
	if n, err := wr.Write([]byte(key)); err != nil || n != len(key) {
		return 0, fmt.Errorf("mapper write err %v r %v", r, err)
	}
	if n, err := wr.Write([]byte(value)); err != nil || n != len(value) {
		return 0, fmt.Errorf("mapper write err %v r %v", r, err)
	}
	if n, err := wr.Write([]byte(jsonPadding)); err != nil || n != len(jsonPadding) {
		return 0, fmt.Errorf("mapper write err %v r %v", r, err)
	}
	return 16 + int(l1) + int(l2) + len(jsonPadding), nil
}

func DecodeKV(rd io.Reader) ([]byte, string, error) {
	var l1 int64
	var l2 int64

	if err := binary.Read(rd, binary.LittleEndian, &l1); err != nil {
		return nil, "", err
	}

	if err := binary.Read(rd, binary.LittleEndian, &l2); err != nil {
		return nil, "", err
	}

	b1 := make([]byte, l1)
	b2 := make([]byte, l2)
	b3 := make([]byte, len(jsonPadding))

	n, err := io.ReadFull(rd, b1)
	if err != nil {
		return nil, "", err
	}
	if n != int(l1) {
		return nil, "", fmt.Errorf("bad string")
	}

	n, err = io.ReadFull(rd, b2)
	if err != nil {
		return nil, "", err
	}
	if n != int(l2) {
		return nil, "", fmt.Errorf("bad string")
	}
	n, err = io.ReadFull(rd, b3)
	if err != nil {
		return nil, "", err
	}
	if n != len(jsonPadding) {
		return nil, "", fmt.Errorf("bad string")
	}
	return b1, string(b2), nil
}
