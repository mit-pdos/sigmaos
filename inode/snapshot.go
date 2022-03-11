package inode

import (
	"encoding/json"
	"log"

	"ulambda/fs"
	np "ulambda/ninep"
)

type InodeSnapshot struct {
	Perm    np.Tperm
	Version np.TQversion
	Mtime   int64
	Parent  uint64
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
	if inode.parent == nil {
		i.Parent = 0
	} else {
		i.Parent = inode.parent.Inum()
	}
	i.Owner = inode.owner

	b, err := json.Marshal(i)
	if err != nil {
		log.Fatalf("Error marshalling inode snapshot: %v", err)
	}
	return b
}

func restoreInode(fn fs.RestoreF, b []byte) fs.Inode {
	i := &InodeSnapshot{}
	err := json.Unmarshal(b, i)
	if err != nil {
		log.Fatalf("FATAL error unmarshal inode in restoreInode: %v", err)
	}
	inode := &Inode{}
	inode.perm = i.Perm
	inode.version = i.Version
	inode.mtime = i.Mtime
	parent := fn(i.Parent)
	if parent != nil {
		inode.parent = parent.(fs.Dir)
	}
	inode.owner = i.Owner
	return inode
}
