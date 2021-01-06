package npcodec

import (
	"encoding/binary"
	"io"
	"log"
)

// Adopted from https://github.com/docker/go-p9p/encoding.go and Go's codecs

func ReadFrame(rd io.Reader) ([]byte, error) {
	var len uint32

	if err := binary.Read(rd, binary.LittleEndian, &len); err != nil {
		log.Fatal("Read error ", err)
		return nil, err
	}
	len = len - 4
	if len <= 0 {
		log.Fatal("readMsg too short")
	}
	msg := make([]byte, len)
	n, err := io.ReadFull(rd, msg)
	if n != int(len) {
		log.Fatal("readFrame error: ", err)
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
		log.Fatal("writeFrame too short")
	}
	return nil
}
