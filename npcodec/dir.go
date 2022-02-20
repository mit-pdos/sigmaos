package npcodec

import (
	np "ulambda/ninep"
)

func DirSize(dir []*np.Stat) np.Tlength {
	sz := uint32(0)
	for _, st := range dir {
		sz += SizeNp(*st)
	}
	return np.Tlength(sz)
}

func Dir2Byte(cnt np.Tsize, dir []*np.Stat) ([]byte, int, *np.Err) {
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

func Byte2Dir(data []byte) ([]*np.Stat, *np.Err) {
	dirents := []*np.Stat{}
	for len(data) > 0 {
		st := np.Stat{}
		if err := Unmarshal(data, &st); err != nil {
			return dirents, err
		}
		dirents = append(dirents, &st)
		sz := np.Tsize(SizeNp(st))
		data = data[sz:]
	}
	return dirents, nil
}
