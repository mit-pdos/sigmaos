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

func MakeRoot() *Root {
	r := Root{}
	r.inode = makeInode(np.DMDIR, RootInum, makeDir())
	r.nextInum = RootInum + 1
	dir := r.inode.Data.(*Dir)
	dir.init(r.inode)
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

func (root *Root) MkNod(uname string, inode *Inode, name string, d Data) (*Inode, error) {
	inode, err := inode.Create(uname, root, np.DMDEVICE, name)
	if err != nil {
		return nil, err
	}
	inode.Data = d
	return inode, nil
}

func (root *Root) MkPipe(uname string, inode *Inode, name string) (*Inode, error) {
	inode, err := inode.Create(uname, root, np.DMNAMEDPIPE, name)
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

func unlockOrdered(olddir *Dir, newdir *Dir) {
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

func (root *Root) Rename(uname string, old []string, new []string) error {
	db.DPrintf("%v: Rename %s to %s\n", uname, old, new)

	rootino := root.inode
	if len(old) == 0 || len(new) == 0 {
		return errors.New("Cannot rename directory")
	}
	oldname := old[len(old)-1]
	newname := new[len(new)-1]
	olddir, ino, err := rootino.LookupPath(uname, old)
	if err != nil {
		return err
	}
	db.DPrintf("%v: Lookup old %v %v %v\n", uname, olddir, ino, err)
	_, i, err := rootino.LookupPath(uname, new[:len(new)-1])
	if err != nil {
		return err
	}
	db.DPrintf("%v: Lookup new %v %v\n", uname, i, err)
	if !i.IsDir() {
		return errors.New("Dst is not a directory")
	}
	newdir := i.Data.(*Dir)

	lockOrdered(olddir, newdir)
	defer unlockOrdered(olddir, newdir)

	// XXX should check if oldname still exists, etc.

	// XXX maybe use inode.Remove()
	db.DPrintf("%v: remove %v from %v\n", uname, oldname, olddir)
	err = olddir.removeLocked(oldname)
	if err != nil {
		log.Fatalf("%v: remove locked %v\n", uname, newname)
	}
	i, err = newdir.lookupLocked(newname)
	if err == nil {
		err = newdir.removeLocked(newname)
		if err != nil {
			log.Fatalf("%v: remove locked %v\n", uname, newname)
		}
		root.freeInum(i.Inum)
	}

	err = newdir.createLocked(ino, newname)
	if err != nil {
		log.Fatalf("%v: Rename createLocked: %v\n", uname, err)
		return err
	}

	db.DPrintf("%v: Rename succeeded %v\n", uname, newdir)
	return nil
}
