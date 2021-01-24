package memfs

import (
	"errors"
	"log"

	db "ulambda/debug"
	np "ulambda/ninep"
)

// XXX need mutex for nextInum
type Root struct {
	inode    *Inode
	nextInum Tinum
}

func MakeRoot(d bool) *Root {
	r := Root{}
	r.inode = makeInode(np.DMDIR, RootInum, makeDir())
	r.nextInum = RootInum + 1
	dir := r.inode.Data.(*Dir)
	dir.init(r.inode)
	db.SetDebug(d)
	return &r
}

// XXX separate out allocator?
func (root *Root) RootInode() *Inode {
	return root.inode
}

// XXX bump version # if allocating same inode #
// XXX a better inum allocation plan
func (root *Root) allocInum() Tinum {
	inum := root.nextInum
	root.nextInum += 1
	return inum
}

func (root *Root) freeInum(inum Tinum) {
}

func (root *Root) MkNod(inode *Inode, name string, d Data) (*Inode, error) {
	inode, err := inode.Create(0, root, np.DMDEVICE, name)
	if err != nil {
		return nil, err
	}
	inode.Data = d
	return inode, nil
}

func (root *Root) MkPipe(inode *Inode, name string) (*Inode, error) {
	inode, err := inode.Create(0, root, np.DMNAMEDPIPE, name)
	if err != nil {
		return nil, err
	}
	return inode, nil
}

func lockOrdered(olddir *Dir, newdir *Dir) {
	if olddir.inum == newdir.inum {
		olddir.mu.Lock()
	} else if olddir.inum < newdir.inum {
		olddir.mu.Lock()
		newdir.mu.Lock()
	} else {
		newdir.mu.Lock()
		olddir.mu.Lock()
	}
}

func unlockOrdered(tid int, olddir *Dir, newdir *Dir) {
	if olddir.inum == newdir.inum {
		olddir.mu.Unlock()
	} else if olddir.inum < newdir.inum {
		olddir.mu.Unlock()
		newdir.mu.Unlock()
	} else {
		newdir.mu.Unlock()
		olddir.mu.Unlock()
	}
}

func (root *Root) Rename(tid int, old []string, new []string) error {
	db.DPrintf("%d: Rename %s to %s\n", tid, old, new)

	rootino := root.inode
	if len(old) == 0 || len(new) == 0 {
		return errors.New("Cannot rename directory")
	}
	oldname := old[len(old)-1]
	newname := new[len(new)-1]
	olddir, ino, err := rootino.LookupPath(tid, old)
	if err != nil {
		return err
	}
	db.DPrintf("Lookup old %v %v %v\n", olddir, ino, err)
	_, i, err := rootino.LookupPath(tid, new[:len(new)-1])
	if err != nil {
		return err
	}
	db.DPrintf("Lookup new %v %v\n", i, err)
	if !i.IsDir() {
		return errors.New("Dst is not a directory")
	}
	newdir := i.Data.(*Dir)

	lockOrdered(olddir, newdir)
	defer unlockOrdered(tid, olddir, newdir)

	// XXX should check if oldname still exists, newname doesn't exist, etc.

	db.DPrintf("remove %v from %v\n", oldname, olddir)
	err = olddir.removeLocked(oldname)
	if err != nil {
		return err
	}

	db.DPrintf("remove succeeded\n")

	err = newdir.createLocked(ino, newname)
	if err != nil {
		log.Print("Rename createLocked: ", err)
	}

	db.DPrintf("rename succeeded %v\n", newdir)
	return nil
}
