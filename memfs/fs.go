package memfs

import (
	"errors"
	"log"

	np "ulambda/ninep"
)

// XXX need mutex
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

// separate out allocator?
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
	inode, err := inode.Create(root, np.DMDEVICE, name)
	if err != nil {
		return nil, err
	}
	inode.Data = d
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

func (root *Root) Rename(old []string, new []string) error {
	log.Printf("Rename %s to %s\n", old, new)
	rootino := root.inode
	if len(old) == 0 || len(new) == 0 {
		return errors.New("Cannot rename directory")
	}
	oldname := old[len(old)-1]
	newname := new[len(new)-1]
	olddir, ino, err := rootino.LookupPath(old)
	if err != nil {
		return err
	}
	newdir, i, err := rootino.LookupPath(new[:len(new)-1])
	if err != nil {
		return err
	}
	if i != nil {
		return errors.New("Dst is not a directory")
	}

	lockOrdered(olddir, newdir)
	defer unlockOrdered(olddir, newdir)

	// XXX should check if oldname still exists, newname doesn't exist, etc.

	err = olddir.removeLocked(oldname)
	if err != nil {
		log.Fatal("Remove error ", err)
	}

	err = newdir.createLocked(ino, newname)
	if err != nil {
		log.Fatal("Create error ", err)
	}
	return nil
}
