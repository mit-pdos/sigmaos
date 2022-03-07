package npcodec

import (
	"io"

	np "ulambda/ninep"
)

func MarshalSizeDir(dir []*np.Stat) np.Tlength {
	sz := uint32(0)
	for _, st := range dir {
		sz += SizeNp(*st)
	}
	return np.Tlength(sz)
}

func MarshalDir(cnt np.Tsize, dir []*np.Stat) ([]byte, int, *np.Err) {
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
		b, err := Marshal(*st)
		if err != nil {
			return nil, n, err
		}
		buf = append(buf, b...)
		cnt -= sz
		n += 1
	}
	return buf, n, nil
}

func UnmarshalDirEnt(rdr io.Reader) (*np.Stat, *np.Err) {
	st := np.Stat{}
	if err := unmarshalReader(rdr, &st); err != nil {
		return nil, err
	}
	return &st, nil
}
