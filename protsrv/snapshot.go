package protsrv

import (
	"encoding/json"

	db "ulambda/debug"
	"ulambda/fid"
	np "ulambda/ninep"
	"ulambda/sesssrv"
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
	for o, f := range fos.et.ephemeral {
		ptr := o.Qid().Path
		foss.Ephemeral[ptr] = f.Snapshot()
	}
	foss.Sid = fos.sid
	b, err := json.Marshal(foss)
	if err != nil {
		db.DFatalf("FATAL Error snapshot encoding fsobjsrv: %v", err)
	}
	return b
}

func Restore(srv np.FsServer, b []byte) np.Protsrv {
	ssrv := srv.(*sesssrv.SessSrv)
	foss := MakeProtSrvSnapshot()
	err := json.Unmarshal(b, foss)
	if err != nil {
		db.DFatalf("FATAL error unmarshal fsobjsrv in restore: %v", err)
	}
	fos := MakeProtServer(ssrv, foss.Sid).(*ProtSrv)
	for f, b := range foss.Fid {
		fos.ft.fids[f] = fid.Restore(ssrv.GetSnapshotter().RestoreFsTree, ssrv.GetSessCondTable(), b)
	}
	for ptr, b := range foss.Ephemeral {
		o := ssrv.GetSnapshotter().RestoreFsTree(ptr)
		fos.et.ephemeral[o] = fid.Restore(ssrv.GetSnapshotter().RestoreFsTree, ssrv.GetSessCondTable(), b)
	}
	return fos
}
