package lazypages

import (
	"path/filepath"

	sp "sigmaos/sigmap"
)

const (
	DIR      = "lazypagesd"
	SOCKNAME = "lazy-pages.socket"
)

func WorkDir(pid sp.Tpid) string {
	return filepath.Join(DIR, pid.String())
}

func SrvPath(pid sp.Tpid) string {
	return filepath.Join(sp.LAZYPAGESD, pid.String())
}
