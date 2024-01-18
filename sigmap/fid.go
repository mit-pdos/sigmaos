package sigmap

import (
	"fmt"
)

func (fid Tfid) String() string {
	if fid == NoFid {
		return "-1"
	}
	return fmt.Sprintf("fid %d", fid)
}
