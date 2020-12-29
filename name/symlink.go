package name

import (
	"ulambda/fid"
)

type Symlink struct {
	start *fid.Ufid
	dst   string
}

func makeSym(start *fid.Ufid, dst string) *Symlink {
	s := Symlink{start, dst}
	return &s
}
