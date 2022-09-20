package protsrv

import (
	"encoding/json"

	db "sigmaos/debug"
	"sigmaos/fid"
	np "sigmaos/ninep"
	"sigmaos/sesssrv"
)

type ProtSrvSnapshot struct {
	Fid       map[np.Tfid][]byte
	Ephemeral map[np.Tpath][]byte
	Sid       np.Tsession
}

func MakeProtSrvSnapshot() *ProtSrvSnapshot {
	foss := &ProtSrvSnapshot{}
	foss.Fid = make(map[np.Tfid][]byte)
	foss.Ephemeral = make(map[np.Tpath][]byte)
	return foss
}

func (fos *ProtSrv) snapshot() []byte {
	foss := MakeProtSrvSnapshot()
	// Snapshot the Fid table.
	for fid, f := range fos.ft.fids {
		foss.Fid[fid] = f.Snapshot()
	}
	for _, po := range fos.et.ephemeral {
		ptr := po.Obj().Path()
		foss.Ephemeral[ptr] = po.Snapshot()
	}
	foss.Sid = fos.sid
	b, err := json.Marshal(foss)
	if err != nil {
		db.DFatalf("Error snapshot encoding fsobjsrv: %v", err)
	}
	return b
}

func Restore(srv np.SessServer, b []byte) np.Protsrv {
	ssrv := srv.(*sesssrv.SessSrv)
	foss := MakeProtSrvSnapshot()
	err := json.Unmarshal(b, foss)
	if err != nil {
		db.DFatalf("error unmarshal fsobjsrv in restore: %v", err)
	}
	fos := MakeProtServer(ssrv, foss.Sid).(*ProtSrv)
	for f, b := range foss.Fid {
		fos.ft.fids[f] = fid.Restore(ssrv.GetSnapshotter().RestoreFsTree, ssrv.GetSessCondTable(), b)
	}
	for ptr, b := range foss.Ephemeral {
		o := ssrv.GetSnapshotter().RestoreFsTree(ptr)
		fos.et.ephemeral[o] = fid.RestorePobj(ssrv.GetSnapshotter().RestoreFsTree, ssrv.GetSessCondTable(), b)
	}
	return fos
}
