// Package memfs implements an in-memory file system. A typical
// sigmaos server has one (e.g., to export its RPC interface).
package memfs

import (
	//"time"
	"fmt"

	"sigmaos/fs"
	"sigmaos/memfs/file"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type File struct {
	fs.Inode
	*file.File
}

func NewFile(i fs.Inode) *File {
	f := &File{}
	f.Inode = i
	f.File = file.NewFile()
	return f
}

func (f *File) String() string {
	return fmt.Sprintf("{%v len %d}", f.Inode, f.File.Size())
}

func (f *File) Size() sp.Tlength {
	return f.File.Size()
}

func (f *File) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	st, err := f.Inode.NewStat()
	if err != nil {
		return nil, err
	}
	l := f.Size()
	st.SetLength(l)
	return st, nil
}
