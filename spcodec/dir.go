package spcodec

import (
	"errors"
	"io"

	// db "sigmaos/debug"
	"sigmaos/fcall"
	sp "sigmaos/sigmap"
)

func MarshalSizeDir(dir []*sp.Stat) sp.Tlength {
	sz := uint64(0)
	for _, st := range dir {
		sz += SizeNp(*st)
	}
	return sp.Tlength(sz)
}

// XXX SizeNp is incorrect
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
