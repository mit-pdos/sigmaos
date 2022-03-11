package realm

import (
	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
)

type CtlFile struct {
	queue chan string
	fs.Inode
}

func makeCtlFile(queue chan string, ctx fs.CtxI, parent fs.Dir) *CtlFile {
	i := inode.MakeInode(ctx, 0, parent)
	return &CtlFile{queue, i}
}

func (ctl *CtlFile) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	return nil, np.MkErr(np.TErrNotSupported, "Read")
}

func (ctl *CtlFile) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	ctl.queue <- string(b)
	return np.Tsize(len(b)), nil
}
