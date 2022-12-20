package npcodec

import (
	"errors"
	"io"

	db "sigmaos/debug"
	"sigmaos/sessp"
	np "sigmaos/ninep"
)

func MarshalSizeDir(dir []*np.Stat9P) np.Tlength {
	sz := uint64(0)
	for _, st := range dir {
		sz += sizeNp(*st)
	}
	return np.Tlength(sz)
}

func MarshalDirEnt(st *np.Stat9P, cnt uint64) ([]byte, *sessp.Err) {
	sz := sizeNp(*st)
	if cnt < sz {
		return nil, nil
	}
	b, e := marshal(*st)
	if e != nil {
		return nil, sessp.MkErrError(e)
	}
	if sz != uint64(len(b)) {
		db.DFatalf("MarshalDirEnt %v %v\n", sz, len(b))
	}
	return b, nil
}

func UnmarshalDirEnt(rdr io.Reader) (*np.Stat9P, *sessp.Err) {
	st := np.Stat9P{}
	if error := unmarshalReader(rdr, &st); error != nil {
		var nperr *sessp.Err
		if errors.As(error, &nperr) {
			return nil, nperr
		}
		return nil, sessp.MkErrError(error)
	}
	return &st, nil
}
