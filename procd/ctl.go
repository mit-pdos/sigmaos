package procd

import (
	"encoding/json"
	"fmt"

	db "sigmaos/debug"
	"sigmaos/sessp"
    "sigmaos/serr"
	"sigmaos/fs"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type SpawnFile struct {
	pd *Procd
	fs.Inode
}

func makeSpawnFile(pd *Procd) *serr.Err {
	sf := &SpawnFile{}
	sf.pd = pd
	sf.Inode = pd.memfssrv.MakeDevInode()
	err := pd.memfssrv.MkDev(sp.PROCD_SPAWN_FILE, sf)
	if err != nil {
		return err
	}
	return nil
}

func (ctl *SpawnFile) Read(ctx fs.CtxI, off sp.Toffset, cnt sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	return nil, serr.MkErr(serr.TErrNotSupported, "Read")
}

func (ctl *SpawnFile) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *serr.Err) {
	p := proc.MakeEmptyProc()
	err := json.Unmarshal(b, p)
	if err != nil {
		serr.MkErr(serr.TErrInval, fmt.Sprintf("Unmarshal %v", err))
	}

	db.DPrintf(db.PROCD, "Control file write: %v", p)

	ctl.pd.fs.spawn(p, b)

	db.DPrintf(db.PROCD, "fs spawn done: %v", p)

	return sessp.Tsize(len(b)), nil
}
