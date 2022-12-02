package spcodec

import (
	"bytes"
	"encoding/binary"
	"io"

	"google.golang.org/protobuf/proto"

	// db "sigmaos/debug"
	"sigmaos/fcall"
	"sigmaos/frame"
	sp "sigmaos/sigmap"
)

func MarshalSizeDir(dir []*sp.Stat) (sp.Tlength, *fcall.Err) {
	sz := uint64(0)
	for _, st := range dir {
		b, err := proto.Marshal(st)
		if err != nil {
			return 0, fcall.MkErrError(err)
		}
		sz += uint64(len(b))
	}
	return sp.Tlength(sz), nil
}

func MarshalDirEnt(st *sp.Stat, cnt uint64) ([]byte, *fcall.Err) {
	var buf bytes.Buffer
	b, err := proto.Marshal(st)
	if err != nil {
		return nil, fcall.MkErrError(err)
	}
	sz := binary.Size(uint64(len(b)))
	if cnt < uint64(len(b)+sz) {
		return nil, nil
	}
	if err := frame.PushToFrame(&buf, b); err != nil {
		return nil, fcall.MkErrError(err)
	}
	return buf.Bytes(), nil
}

func UnmarshalDirEnt(rdr io.Reader) (*sp.Stat, *fcall.Err) {
	st := sp.MkStatNull()
	b, err := frame.PopFromFrame(rdr)
	if err != nil {
		return nil, fcall.MkErrError(err)
	}
	if err := proto.Unmarshal(b, st); err != nil {
		return nil, fcall.MkErrError(err)
	}
	//var nperr *fcall.Err
	//if errors.As(error, &nperr) {
	//		return nil, nperr
	//	}
	//	return nil, fcall.MkErrError(error)
	return st, nil
}
