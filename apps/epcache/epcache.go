package epcache

import (
	"fmt"
)

const (
	EPCACHEREL = "epcache"
	EPCACHE    = "name/" + EPCACHEREL
)

const (
	NO_VERSION Tversion = ^Tversion(0)
)

type Tversion uint64

func (v Tversion) String() string {
	if v == NO_VERSION {
		return "vNONE"
	}
	return fmt.Sprintf("v%v", uint64(v))
}
