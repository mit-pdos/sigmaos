package npcodec

import (
	"errors"
	"io"

	db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/serr"
)

func MarshalSizeDir(dir []*np.Stat9P) np.Tlength {
	sz := uint64(0)
	for _, st := range dir {
		sz += sizeNp(*st)
	}
	return np.Tlength(sz)
}

func MarshalDirEnt(st *np.Stat9P, cnt uint64) ([]byte, *serr.Err) {
	sz := sizeNp(*st)
	if cnt < sz {
		return nil, nil
	}
	b, e := marshal(*st)
	if e != nil {
		return nil, serr.NewErrError(e)
	}
	if sz != uint64(len(b)) {
		db.DFatalf("MarshalDirEnt %v %v\n", sz, len(b))
	}
	return b, nil
}

func UnmarshalDirEnt(rdr io.Reader) (*np.Stat9P, *serr.Err) {
	st := np.Stat9P{}
	if error := unmarshalReader(rdr, &st); error != nil {
		var nperr *serr.Err
		if errors.As(error, &nperr) {
			return nil, nperr
		}
		return nil, serr.NewErrError(error)
	}
	return &st, nil
}
