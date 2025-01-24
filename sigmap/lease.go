package sigmap

import (
	"strconv"
)

func (lid TleaseId) IsLeased() bool {
	return lid != NoLeaseId
}

func (lid TleaseId) String() string {
	if lid == NoLeaseId {
		return "-1"
	} else {
		return strconv.FormatUint(uint64(lid), 16)
	}
}
