package sigmap

import (
	"strconv"
)

func (cid TclntId) String() string {
	return strconv.FormatUint(uint64(cid), 16)
}
