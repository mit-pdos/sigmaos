package sessdev

import (
	"path"
)

const (
	DATA  = "-data"
	CTL   = "-ctl"
	CLONE = "-clone"
)

func CloneName(fn string) string {
	return fn + CLONE
}

func SidName(sid string, pn string) string {
	return pn + "-" + sid
}

func DataName(pn string) string {
	return path.Base(pn) + DATA
}
