package procd

import (
	"encoding/json"
	"fmt"

	db "sigmaos/debug"
	"sigmaos/fs"
	np "sigmaos/sigmap"
	"sigmaos/proc"
)

type SpawnFile struct {
	pd *Procd
	fs.Inode
}

func makeSpawnFile(pd *Procd) *np.Err {
	sp := &SpawnFile{}
	sp.pd = pd
	sp.Inode = pd.memfssrv.MakeDevInode()
	err := pd.memfssrv.MkDev(np.PROCD_SPAWN_FILE, sp)
	if err != nil {
		return err
	}
	return nil
}

func (ctl *SpawnFile) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	return nil, np.MkErr(np.TErrNotSupported, "Read")
}

func (ctl *SpawnFile) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	p := proc.MakeEmptyProc()
	err := json.Unmarshal(b, p)
	if err != nil {
		np.MkErr(np.TErrInval, fmt.Sprintf("Unmarshal %v", err))
	}

	db.DPrintf("PROCD", "Control file write: %v", p)

	ctl.pd.fs.spawn(p, b)

	db.DPrintf("PROCD", "fs spawn done: %v", p)

	return np.Tsize(len(b)), nil
}
