package inode

import (
	"encoding/json"

	db "sigmaos/debug"
	"sigmaos/fs"
	np "sigmaos/ninep"
)

type InodeSnapshot struct {
	Perm   np.Tperm
	Mtime  int64
	Parent np.Tpath
	Owner  string
	Nlink  int
}

func makeSnapshot(inode *Inode) []byte {
	i := &InodeSnapshot{}
	i.Perm = inode.perm
	i.Mtime = 0 // TODO: decide what to do about time.
	// Since we traverse down the tree, we assume the parent must have already
	// been snapshotted.
	if inode.parent == nil {
		i.Parent = 0
	} else {
		i.Parent = inode.parent.Path()
	}
	i.Owner = inode.owner

	b, err := json.Marshal(i)
	if err != nil {
		db.DFatalf("Error marshalling inode snapshot: %v", err)
	}
	return b
}

func restoreInode(fn fs.RestoreF, b []byte) fs.Inode {
	i := &InodeSnapshot{}
	err := json.Unmarshal(b, i)
	if err != nil {
		db.DFatalf("error unmarshal inode in restoreInode: %v", err)
	}
	inode := &Inode{}
	inode.perm = i.Perm
	inode.mtime = i.Mtime
	parent := fn(i.Parent)
	if parent != nil {
		inode.parent = parent.(fs.Dir)
	}
	inode.owner = i.Owner
	return inode
}
