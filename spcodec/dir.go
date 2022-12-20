package spcodec

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

func MarshalSizeDir(dir []*sp.Stat) (sp.Tlength, *serr.Err) {
	sz := uint64(0)
	for _, st := range dir {
		b, err := proto.Marshal(st)
		if err != nil {
			return 0, serr.MkErrError(err)
		}
		sz += uint64(len(b))
	}
	return sp.Tlength(sz), nil
}

func MarshalDirEnt(st *sp.Stat, cnt uint64) ([]byte, *serr.Err) {
	var buf bytes.Buffer
	b, err := proto.Marshal(st)
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	sz := binary.Size(uint64(len(b)))
	if cnt < uint64(len(b)+sz) {
		return nil, nil
	}
	if err := frame.PushToFrame(&buf, b); err != nil {
		return nil, serr.MkErrError(err)
	}
	return buf.Bytes(), nil
}

func UnmarshalDirEnt(rdr io.Reader) (*sp.Stat, *serr.Err) {
	st := sp.MkStatNull()
	b, err := frame.PopFromFrame(rdr)
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	if err := proto.Unmarshal(b, st); err != nil {
		return nil, serr.MkErrError(err)
	}
	//var nperr *serr.Err
	//if errors.As(error, &nperr) {
	//		return nil, nperr
	//	}
	//	return nil, serr.MkErrError(error)
	return st, nil
}
