package inode

import (
	"encoding/json"
	"log"
	//	"unsafe"

	np "ulambda/ninep"
)

type InodeSnapshot struct {
	Perm    np.Tperm
	Version np.TQversion
	Mtime   int64
	Parent  uintptr
	Owner   string
	Nlink   int
}

func makeSnapshot(inode *Inode) []byte {
	i := &InodeSnapshot{}
	i.Perm = inode.perm
	i.Version = inode.version
	i.Mtime = 0 // TODO: decide what to do about time.
	// Since we traverse down the tree, we assume the parent must have already
	// been snapshotted.
	i.Parent = 0 // uintptr(unsafe.Pointer(inode.parent.(*dir.DirImpl))) // TODO: How do I get the raw pointer from an interface?
	i.Owner = inode.owner
	i.Nlink = inode.nlink

	b, err := json.Marshal(i)
	if err != nil {
		log.Fatalf("Error marshalling inode snapshot: %v", err)
	}
	return b
}
