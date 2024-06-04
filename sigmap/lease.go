package sigmap

import (
	"strconv"
)

func (lid TleaseId) IsLeased() bool {
	return lid != NoLeaseId
}

func (lid TleaseId) String() string {
	return strconv.FormatUint(uint64(lid), 16)
}
