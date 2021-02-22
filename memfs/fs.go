package memfs

import (
	"errors"
	"fmt"
	"log"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
)

// XXX need mutex for nextInum
type Root struct {
	inode    *Inode
	mu       sync.Mutex
	nextInum Tinum
}

func MakeRoot() *Root {
	r := Root{}
	r.inode = makeInode(np.DMDIR, RootInum, makeDir())
	r.nextInum = RootInum + 1
	dir := r.inode.Data.(*Dir)
	dir.init(r.inode)
	// db.SetDebug(true)
	return &r
}

// XXX separate out allocator?
func (root *Root) RootInode() *Inode {
	return root.inode
}

// XXX bump version # if allocating same inode #
// XXX a better inum allocation plan
func (root *Root) allocInum() Tinum {
	root.mu.Lock()
	defer root.mu.Unlock()

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

func (root *Root) Rename(uname string, old []string, new []string) error {
	db.DPrintf("%v: Rename %s to %s\n", uname, old, new)
	rootino := root.inode
	if len(old) == 0 || len(new) == 0 {
		return errors.New("Cannot rename directory")
	}
	oldname := old[len(old)-1]
	newname := new[len(new)-1]
	dir, err := rootino.LookupPath(uname, old)
	if err != nil {
		return err
	}
	if dir == nil {
		log.Fatalf("No parent directory %v", old)
	}
	dir.mu.Lock()
	defer dir.mu.Unlock()

	db.DPrintf("%v: rename %v from %v\n", uname, oldname, dir)
	ino, err := dir.lookupLocked(oldname)
	if err != nil {
		return fmt.Errorf("Unknown name %v", oldname)
	}
	err = dir.removeLocked(oldname)
	if err != nil {
		log.Fatalf("%v: remove locked %v\n", uname, newname)
	}
	i, err := dir.lookupLocked(newname)
	if err == nil { // i is valid
		// XXX 9p: it is an error to change the name to that
		// of an existing file.
		err = dir.removeLocked(newname)
		if err != nil {
			log.Fatalf("%v: remove locked %v\n", uname, newname)
		}
		root.freeInum(i.Inum)
	}

	err = dir.createLocked(ino, newname)
	if err != nil {
		log.Fatalf("%v: Rename createLocked: %v\n", uname, err)
		return err
	}

	db.DPrintf("%v: Rename succeeded %v %v\n", uname, old, new)
	return nil
}
