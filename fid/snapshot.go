package fid

import (
	"encoding/json"
	"log"
	"reflect"
	"unsafe"

	np "ulambda/ninep"
)

type FidSnapshot struct {
	Path    []string
	Obj     unsafe.Pointer
	IsOpen  bool
	M       np.Tmode
	Vers    np.TQversion
	CtxSnap []byte
	Cursor  int // for directories
}

func MakeFidSnapshot() *FidSnapshot {
	fs := &FidSnapshot{}
	return fs
}

func (fid *Fid) Snapshot() []byte {
	fs := MakeFidSnapshot()
	fs.Path = fid.path
	fs.Obj = unsafe.Pointer(reflect.ValueOf(fid.obj).Pointer())
	fs.M = fid.m
	fs.Vers = fid.vers
	fs.CtxSnap = fid.ctx.Snapshot()
	fs.Cursor = fid.cursor
	b, err := json.Marshal(fs)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding fid: %v", err)
	}
	return b
}
