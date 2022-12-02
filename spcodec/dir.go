package spcodec

import (
	"errors"
	"io"

	"google.golang.org/protobuf/proto"

	// db "sigmaos/debug"
	"sigmaos/fcall"
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

// XXX Cut SizeN[ and pass cnt to marshal/encode?  Or call protobuf.Marshal?
func MarshalDirEnt(st *sp.Stat, cnt uint64) ([]byte, *fcall.Err) {
	sz := SizeNp(*st)
	if cnt < sz {
		return nil, nil
	}
	b, e := marshal(*st)
	if e != nil {
		return nil, fcall.MkErrError(e)
	}
	//if sz != uint64(len(b)) {
	//	db.DFatalf("MarshalDirEnt %v %v\n", sz, len(b))
	//}
	return b, nil
}

func UnmarshalDirEnt(rdr io.Reader) (*sp.Stat, *fcall.Err) {
	st := sp.Stat{}
	if error := unmarshalReader(rdr, &st); error != nil {
		var nperr *fcall.Err
		if errors.As(error, &nperr) {
			return nil, nperr
		}
		return nil, fcall.MkErrError(error)
	}
	return &st, nil
}
