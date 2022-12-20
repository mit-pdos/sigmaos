package procd

import (
	"encoding/json"
	"fmt"

	db "sigmaos/debug"
	"sigmaos/fcall"
	"sigmaos/fs"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type SpawnFile struct {
	pd *Procd
	fs.Inode
}

func makeSpawnFile(pd *Procd) *fcall.Err {
	sf := &SpawnFile{}
	sf.pd = pd
	sf.Inode = pd.memfssrv.MakeDevInode()
	err := pd.memfssrv.MkDev(sp.PROCD_SPAWN_FILE, sf)
	if err != nil {
		return err
	}
	return nil
}

func (ctl *SpawnFile) Read(ctx fs.CtxI, off sp.Toffset, cnt sp.Tsize, v sp.TQversion) ([]byte, *fcall.Err) {
	return nil, fcall.MkErr(fcall.TErrNotSupported, "Read")
}

func (ctl *SpawnFile) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sp.Tsize, *fcall.Err) {
	p := proc.MakeEmptyProc()
	err := json.Unmarshal(b, p)
	if err != nil {
		fcall.MkErr(fcall.TErrInval, fmt.Sprintf("Unmarshal %v", err))
	}

	db.DPrintf(db.PROCD, "Control file write: %v", p)

	ctl.pd.fs.spawn(p, b)

	db.DPrintf(db.PROCD, "fs spawn done: %v", p)

	return sp.Tsize(len(b)), nil
}
