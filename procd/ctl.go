package procd

import (
	"encoding/json"
	"fmt"
	"path"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/semclnt"
)

type CtlFile struct {
	pd *Procd
	fs.Inode
}

func makeCtlFile(pd *Procd, ctx fs.CtxI, parent fs.Dir) *CtlFile {
	i := inode.MakeInode(ctx, 0, parent)
	return &CtlFile{pd, i}
}

func (ctl *CtlFile) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	return nil, np.MkErr(np.TErrNotSupported, "Read")
}

func (ctl *CtlFile) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	p := proc.MakeEmptyProc()
	err := json.Unmarshal(b, p)
	if err != nil {
		np.MkErr(np.TErrInval, fmt.Sprintf("Unmarshal %v", err))
	}

	db.DLPrintf("PROCD", "Control file write: %v", p)

	// Create an ephemeral semaphore to indicate a proc has started.
	semStart := semclnt.MakeSemClnt(ctl.pd.FsLib, path.Join(p.ParentDir, proc.START_SEM))
	semStart.Init(np.DMTMP)

	ctl.pd.fs.spawn(p, b)

	return np.Tsize(len(b)), nil
}
