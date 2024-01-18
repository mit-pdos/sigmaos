package sigmap

import (
	"strconv"
)

func (p Tpath) String() string {
	return strconv.FormatUint(uint64(p), 16)
}

func String2Path(path string) (Tpath, error) {
	p, err := strconv.ParseUint(path, 16, 64)
	if err != nil {
		return Tpath(p), err
	}
	return Tpath(p), nil
}
