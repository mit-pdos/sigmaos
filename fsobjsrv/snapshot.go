package fsobjsrv

import (
	"encoding/json"
	"log"

	"ulambda/fid"
	"ulambda/fssrv"
	np "ulambda/ninep"
	"ulambda/protsrv"
)

type FsObjSrvSnapshot struct {
	Fid       map[np.Tfid][]byte
	Ephemeral map[np.Tpath][]byte
	Sid       np.Tsession
}

func MakeFsObjSrvSnapshot() *FsObjSrvSnapshot {
	foss := &FsObjSrvSnapshot{}
	foss.Fid = make(map[np.Tfid][]byte)
	foss.Ephemeral = make(map[np.Tpath][]byte)
	return foss
}

func (fos *FsObjSrv) snapshot() []byte {
	foss := MakeFsObjSrvSnapshot()
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
		log.Fatalf("FATAL Error snapshot encoding fsobjsrv: %v", err)
	}
	return b
}

func Restore(srv protsrv.FsServer, b []byte) protsrv.Protsrv {
	fssrv := srv.(*fssrv.FsServer)
	foss := MakeFsObjSrvSnapshot()
	err := json.Unmarshal(b, foss)
	if err != nil {
		log.Fatalf("FATAL error unmarshal fsobjsrv in restore: %v", err)
	}
	fos := MakeProtServer(fssrv, foss.Sid).(*FsObjSrv)
	for f, b := range foss.Fid {
		fos.ft.fids[f] = fid.Restore(fssrv.GetSnapshotter().RestoreFsTree, fssrv.GetSessCondTable(), b)
	}
	for ptr, b := range foss.Ephemeral {
		o := fssrv.GetSnapshotter().RestoreFsTree(ptr)
		fos.et.ephemeral[o] = fid.Restore(fssrv.GetSnapshotter().RestoreFsTree, fssrv.GetSessCondTable(), b)
	}
	return fos
}
