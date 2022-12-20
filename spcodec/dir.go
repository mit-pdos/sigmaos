package spcodec

import (
	"bytes"
	"encoding/binary"
	"io"

	"google.golang.org/protobuf/proto"

	// db "sigmaos/debug"
	"sigmaos/sessp"
	"sigmaos/frame"
	sp "sigmaos/sigmap"
)

func MarshalSizeDir(dir []*sp.Stat) (sp.Tlength, *sessp.Err) {
	sz := uint64(0)
	for _, st := range dir {
		b, err := proto.Marshal(st)
		if err != nil {
			return 0, sessp.MkErrError(err)
		}
		sz += uint64(len(b))
	}
	return sp.Tlength(sz), nil
}

func MarshalDirEnt(st *sp.Stat, cnt uint64) ([]byte, *sessp.Err) {
	var buf bytes.Buffer
	b, err := proto.Marshal(st)
	if err != nil {
		return nil, sessp.MkErrError(err)
	}
	sz := binary.Size(uint64(len(b)))
	if cnt < uint64(len(b)+sz) {
		return nil, nil
	}
	if err := frame.PushToFrame(&buf, b); err != nil {
		return nil, sessp.MkErrError(err)
	}
	return buf.Bytes(), nil
}

func UnmarshalDirEnt(rdr io.Reader) (*sp.Stat, *sessp.Err) {
	st := sp.MkStatNull()
	b, err := frame.PopFromFrame(rdr)
	if err != nil {
		return nil, sessp.MkErrError(err)
	}
	if err := proto.Unmarshal(b, st); err != nil {
		return nil, sessp.MkErrError(err)
	}
	//var nperr *sessp.Err
	//if errors.As(error, &nperr) {
	//		return nil, nperr
	//	}
	//	return nil, sessp.MkErrError(error)
	return st, nil
}
