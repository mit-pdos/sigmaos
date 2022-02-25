package inode

import (
	"encoding/json"
	"log"
	"reflect"

	"ulambda/fs"
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
	i.Parent = reflect.ValueOf(inode.parent).Pointer()
	i.Owner = inode.owner
	i.Nlink = inode.nlink

	b, err := json.Marshal(i)
	if err != nil {
		log.Fatalf("Error marshalling inode snapshot: %v", err)
	}
	return b
}

func restoreInode(fn fs.RestoreF, b []byte) fs.FsObj {
	i := &InodeSnapshot{}
	err := json.Unmarshal(b, i)
	if err != nil {
		log.Fatalf("FATAL error unmarshal inode in restoreInode: %v", err)
	}
	inode := &Inode{}
	inode.perm = i.Perm
	inode.version = i.Version
	inode.mtime = i.Mtime
	inode.parent = fn(i.Parent).(fs.Dir)
	inode.owner = i.Owner
	inode.nlink = i.Nlink
	return inode
}
