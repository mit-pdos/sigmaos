package procd

import (
	"encoding/json"
	"fmt"
	"log"

	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
	"ulambda/proc"
)

type CtlFile struct {
	pd *Procd
	fs.FsObj
}

func makeCtlFile(pd *Procd, uname string, parent fs.Dir) *CtlFile {
	i := inode.MakeInode(uname, 0, parent)
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

	ctl.pd.fs.pubSpawned(p, b)

	return np.Tsize(len(b)), nil
}
