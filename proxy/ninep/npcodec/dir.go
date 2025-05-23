package npcodec

import (
	"io"

	db "sigmaos/debug"
	np "sigmaos/ninep"
)

func MarshalSizeDir(dir []*np.Stat9P) np.Tlength {
	sz := uint64(0)
	for _, st := range dir {
		sz += sizeNp(*st)
	}
	return np.Tlength(sz)
}

func MarshalDirEnt(st *np.Stat9P, cnt uint64) ([]byte, error) {
	sz := sizeNp(*st)
	if cnt < sz {
		return nil, nil
	}
	b, e := marshal(*st)
	if e != nil {
		return nil, e
	}
	if sz != uint64(len(b)) {
		db.DFatalf("MarshalDirEnt %v %v\n", sz, len(b))
	}
	return b, nil
}

func UnmarshalDirEnt(rdr io.Reader) (*np.Stat9P, error) {
	st := np.Stat9P{}
	if err := unmarshalReader(rdr, &st); err != nil {
		return nil, err
	}
	return &st, nil
}
