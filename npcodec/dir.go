package npcodec

import (
	"sort"

	np "ulambda/ninep"
)

func DirSize(dir []*np.Stat) np.Tlength {
	sz := uint32(0)
	for _, st := range dir {
		sz += SizeNp(*st)
	}
	return np.Tlength(sz)
}

// Marshall part  of a directory [offset, cnt)
func Dir2Buf(offset np.Toffset, cnt np.Tsize, dir []*np.Stat) ([]byte, error) {
	var buf []byte

	if offset >= np.Toffset(DirSize(dir)) {
		return nil, nil
	}

	// sort dir by st.Name
	sort.SliceStable(dir, func(i, j int) bool {
		return dir[i].Name < dir[j].Name
	})

	off := np.Toffset(0)
	for _, st := range dir {
		sz := np.Tsize(SizeNp(*st))
		if cnt < sz {
			break
		}
		if off >= offset {
			b, err := Marshal(*st)
			if err != nil {
				return nil, err
			}
			buf = append(buf, b...)
			cnt -= sz
		}
		off += np.Toffset(sz)
	}
	return buf, nil
}
