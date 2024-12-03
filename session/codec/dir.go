package codec

import (
	"bytes"
	"encoding/binary"
	"io"

	"google.golang.org/protobuf/proto"

	// db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func MarshalSizeDir(dir []*sp.Tstat) (sp.Tlength, *serr.Err) {
	sz := uint64(0)
	for _, st := range dir {
		b, err := proto.Marshal(st)
		if err != nil {
			return 0, serr.NewErrError(err)
		}
		sz += uint64(len(b))
	}
	return sp.Tlength(sz), nil
}

func MarshalDirEnt(st *sp.Tstat, cnt uint64) ([]byte, *serr.Err) {
	var buf bytes.Buffer
	b, err := proto.Marshal(st)
	if err != nil {
		return nil, serr.NewErrError(err)
	}
	sz := binary.Size(uint32(len(b)))
	if cnt < uint64(len(b)+sz) {
		return nil, nil
	}
	if err := frame.WriteFrame(&buf, b); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func UnmarshalDirEnt(rdr io.Reader) (*sp.Tstat, *serr.Err) {
	st := sp.NewStatNull()
	b, err := frame.ReadFrame(rdr)
	if err != nil {
		return nil, err
	}
	if err := proto.Unmarshal(b, st); err != nil {
		return nil, serr.NewErrError(err)
	}
	return st, nil
}
