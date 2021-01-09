package npcodec

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Adopted from https://github.com/docker/go-p9p/encoding.go and Go's codecs

func ReadFrame(rd io.Reader) ([]byte, error) {
	var len uint32

	if err := binary.Read(rd, binary.LittleEndian, &len); err != nil {
		return nil, err
	}
	len = len - 4
	if len <= 0 {
		return nil, errors.New("readMsg too short")
	}
	msg := make([]byte, len)
	n, err := io.ReadFull(rd, msg)
	if n != int(len) {
		return nil, fmt.Errorf("readFrame error: %v", err)
	}
	return msg, err
}

func WriteFrame(wr io.Writer, frame []byte) error {
	l := uint32(len(frame) + 4)

	if err := binary.Write(wr, binary.LittleEndian, l); err != nil {
		return err
	}
	if n, err := wr.Write(frame); err != nil {
		return err
	} else if n < len(frame) {
		errors.New("writeFrame too short")
	}
	return nil
}
