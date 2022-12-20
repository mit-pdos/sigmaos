package procd

import (
	"encoding/json"
	"fmt"

	db "sigmaos/debug"
	"sigmaos/sessp"
	"sigmaos/fs"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type SpawnFile struct {
	pd *Procd
	fs.Inode
}

func makeSpawnFile(pd *Procd) *sessp.Err {
	sf := &SpawnFile{}
	sf.pd = pd
	sf.Inode = pd.memfssrv.MakeDevInode()
	err := pd.memfssrv.MkDev(sp.PROCD_SPAWN_FILE, sf)
	if err != nil {
		return err
	}
	return nil
}

func (ctl *SpawnFile) Read(ctx fs.CtxI, off sp.Toffset, cnt sessp.Tsize, v sp.TQversion) ([]byte, *sessp.Err) {
	return nil, sessp.MkErr(sessp.TErrNotSupported, "Read")
}

func (ctl *SpawnFile) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *sessp.Err) {
	p := proc.MakeEmptyProc()
	err := json.Unmarshal(b, p)
	if err != nil {
		sessp.MkErr(sessp.TErrInval, fmt.Sprintf("Unmarshal %v", err))
	}

	db.DPrintf(db.PROCD, "Control file write: %v", p)

	ctl.pd.fs.spawn(p, b)

	db.DPrintf(db.PROCD, "fs spawn done: %v", p)

	return sessp.Tsize(len(b)), nil
}
