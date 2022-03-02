package fsobjsrv

import (
	"encoding/json"
	"log"
	"reflect"
	"unsafe"

	np "ulambda/ninep"
	"ulambda/protsrv"
)

type FsObjSrvSnapshot struct {
	Fid       map[np.Tfid][]byte
	Ephemeral map[unsafe.Pointer][]byte
	Sid       np.Tsession
}

func MakeFsObjSrvSnapshot() *FsObjSrvSnapshot {
	foss := &FsObjSrvSnapshot{}
	foss.Fid = make(map[np.Tfid][]byte)
	foss.Ephemeral = make(map[unsafe.Pointer][]byte)
	return foss
}

func (fos *FsObjSrv) Snapshot() []byte {
	foss := MakeFsObjSrvSnapshot()
	// Snapshot the Fid table.
	for fid, f := range fos.ft.fids {
		foss.Fid[fid] = f.Snapshot()
	}
	for o, f := range fos.et.ephemeral {
		ptr := unsafe.Pointer(reflect.ValueOf(o).Pointer())
		foss.Ephemeral[ptr] = f.Snapshot()
	}
	foss.Sid = fos.sid
	b, err := json.Marshal(foss)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding fsobjsrv: %v", err)
	}
	return b
}

func Restore(b []byte) protsrv.Protsrv {
	return nil
}
