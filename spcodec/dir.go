package spcodec

import (
	"errors"
	"io"

	"sigmaos/fcall"
	sp "sigmaos/sigmap"
)

func MarshalSizeDir(dir []*sp.Stat) sp.Tlength {
	sz := uint32(0)
	for _, st := range dir {
		sz += SizeNp(*st)
	}
	return sp.Tlength(sz)
}

func MarshalDir(cnt sp.Tsize, dir []*sp.Stat) ([]byte, int, *fcall.Err) {
	var buf []byte

	if len(dir) == 0 {
		return nil, 0, nil
	}
	n := 0
	for _, st := range dir {
		sz := sp.Tsize(SizeNp(*st))
		if cnt < sz {
			break
		}
		b, e := marshal(*st)
		if e != nil {
			return nil, n, fcall.MkErrError(e)
		}
		buf = append(buf, b...)
		cnt -= sz
		n += 1
	}
	return buf, n, nil
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
