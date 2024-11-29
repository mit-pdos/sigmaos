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

type kvdecoder struct {
	rd      io.Reader
	keylen  int
	key     []byte
	value   []byte
	padding []byte
}

func newKVDecoder(rd io.Reader, maxkey, maxvalue int) *kvdecoder {
	return &kvdecoder{
		rd:      rd,
		keylen:  maxkey,
		key:     make([]byte, 0, maxkey),
		value:   make([]byte, 0, maxvalue),
		padding: make([]byte, 0, len(jsonPadding)),
	}
}

func (kvd *kvdecoder) decode() ([]byte, string, error) {
	var l1 int64
	var l2 int64

	if err := binary.Read(kvd.rd, binary.LittleEndian, &l1); err != nil {
		return nil, "", err
	}

	if err := binary.Read(kvd.rd, binary.LittleEndian, &l2); err != nil {
		return nil, "", err
	}

	// Resize read buffer if necessary
	if int(l1) > kvd.keylen {
		for ; int(l1) > kvd.keylen; kvd.keylen *= 2 {
		}
		kvd.key = make([]byte, 0, kvd.keylen)
	}

	kvd.key = kvd.key[:l1]
	kvd.value = kvd.value[:l2]
	kvd.padding = kvd.padding[:len(jsonPadding)]

	n, err := io.ReadFull(kvd.rd, kvd.key)
	if err != nil {
		return nil, "", err
	}
	if n != int(l1) {
		return nil, "", fmt.Errorf("bad string")
	}

	n, err = io.ReadFull(kvd.rd, kvd.value)
	if err != nil {
		return nil, "", err
	}
	if n != int(l2) {
		return nil, "", fmt.Errorf("bad string")
	}
	n, err = io.ReadFull(kvd.rd, kvd.padding)
	if err != nil {
		return nil, "", err
	}
	if n != len(jsonPadding) {
		return nil, "", fmt.Errorf("bad string")
	}
	return kvd.key, string(kvd.value), nil
}
