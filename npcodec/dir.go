package npcodec

import (
	"errors"
	"io"

	"sigmaos/fcall"
	np "sigmaos/sigmap"
)

func MarshalSizeDir(dir []*np.Stat) np.Tlength {
	sz := uint32(0)
	for _, st := range dir {
		sz += SizeNp(*st)
	}
	return np.Tlength(sz)
}

func MarshalDir(cnt np.Tsize, dir []*np.Stat) ([]byte, int, *fcall.Err) {
	var buf []byte

	if len(dir) == 0 {
		return nil, 0, nil
	}
	n := 0
	for _, st := range dir {
		sz := np.Tsize(SizeNp(*st))
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

func UnmarshalDirEnt(rdr io.Reader) (*np.Stat, *fcall.Err) {
	st := np.Stat{}
	if error := unmarshalReader(rdr, &st); error != nil {
		var nperr *fcall.Err
		if errors.As(error, &nperr) {
			return nil, nperr
		}
		return nil, fcall.MkErrError(error)
	}
	return &st, nil
}
