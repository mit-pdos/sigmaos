package procd

import (
	"encoding/json"
	"fmt"
	"log"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
	"ulambda/proc"
)

type CtlFile struct {
	pd *Procd
	fs.FsObj
}

func makeCtlFile(pd *Procd, ctx fs.CtxI, parent fs.Dir) *CtlFile {
	i := inode.MakeInode(ctx, 0, parent)
	return &CtlFile{pd, i}
}

func (ctl *CtlFile) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, error) {
	return nil, fmt.Errorf("not supported")
}

func (ctl *CtlFile) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	p := proc.MakeEmptyProc()
	err := json.Unmarshal(b, p)
	if err != nil {
		log.Fatalf("Couldn't unmarshal proc file in CtlFile.Write: %v, %v", string(b), err)
	}

	db.DLPrintf("PROCD", "Control file write: %v", p)

	ctl.pd.fs.spawn(p, b)

	return np.Tsize(len(b)), nil
}
