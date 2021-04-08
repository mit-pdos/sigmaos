package memfs

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
	"unsafe"

	db "ulambda/debug"
	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
)

type Tinum uint64
type Tversion uint32

type InodeI interface {
	Lock()
	Unlock()
	Perm() np.Tperm
	Qid() np.Tqid
	Version() np.TQversion
	Size() np.Tlength
	Open(npo.CtxI, np.Tmode) error
	Close(npo.CtxI, np.Tmode) error
	Remove(npo.CtxI, string) error
	Stat(npo.CtxI) (*np.Stat, error)
	Rename(npo.CtxI, string, string) error
}

type Inode struct {
	mu      sync.Mutex
	perm    np.Tperm
	version np.TQversion
	Mtime   int64
	parent  *Dir
	owner   string
}

func makeInode(owner string, p np.Tperm, parent *Dir) *Inode {
	i := Inode{}
	i.perm = p
	i.Mtime = time.Now().Unix()
	i.parent = parent
	i.owner = owner
	i.version = np.TQversion(1)
	return &i
}

func (inode *Inode) String() string {
	str := fmt.Sprintf("Inode %p %v", inode, inode.perm)
	return str
}

func (inode *Inode) Lock() {
	inode.mu.Lock()
}

func (inode *Inode) Unlock() {
	inode.mu.Unlock()
}

func (inode *Inode) qidL() np.Tqid {
	id := uintptr(unsafe.Pointer(inode))

	return np.MakeQid(
		np.Qtype(inode.perm>>np.QTYPESHIFT),
		np.TQversion(inode.version),
		np.Tpath(uint64(id)))
}

func (inode *Inode) Qid() np.Tqid {
	inode.Lock()
	defer inode.Unlock()
	return inode.qidL()
}

func (inode *Inode) Perm() np.Tperm {
	inode.mu.Lock()
	defer inode.mu.Unlock()
	return inode.perm
}

func (inode *Inode) Parent() *Dir {
	return inode.parent
}

func (inode *Inode) Version() np.TQversion {
	inode.mu.Lock()
	defer inode.mu.Unlock()
	return inode.version
}

func permToInode(uname string, p np.Tperm, parent *Dir) (InodeI, error) {
	i := makeInode(uname, p, parent)
	if p.IsDir() {
		return makeDir(i), nil
	} else if p.IsSymlink() {
		return MakeSym(i), nil
	} else if p.IsPipe() {
		return MakePipe(i), nil
	} else if p.IsDevice() {
		return MakeDev(i), nil
	} else {
		return MakeFile(i), nil
	}
}

func (i *Inode) Open(ctx npo.CtxI, mode np.Tmode) error {
	return nil
}

func (i *Inode) Close(ctx npo.CtxI, mode np.Tmode) error {
	return nil
}

func (inode *Inode) Mode() np.Tperm {
	perm := np.Tperm(0777)
	if inode.perm.IsDir() {
		perm |= np.DMDIR
	}
	return perm
}

func (inode *Inode) stat() *np.Stat {
	stat := &np.Stat{}
	stat.Type = 0 // XXX
	stat.Qid = inode.qidL()
	stat.Mode = inode.Mode()
	stat.Mtime = uint32(inode.Mtime)
	stat.Atime = 0
	stat.Name = ""
	stat.Uid = inode.owner
	stat.Gid = inode.owner
	stat.Muid = ""
	return stat
}

func (inode *Inode) Remove(ctx npo.CtxI, n string) error {
	inode.Lock()
	defer inode.Unlock()

	db.DLPrintf("MEMFS", "Remove: %v\n", n)

	if inode.parent == nil {
		return errors.New("Cannot remove root directory")
	}
	dir := inode.parent
	dir.Lock()
	defer dir.Unlock()

	_, err := dir.lookupL(n)
	if err != nil {
		return err
	}
	inode.version += 1
	dir.version += 1

	err = dir.removeL(n)
	if err != nil {
		log.Fatalf("Remove: error %v\n", n)
	}

	return nil
}

func (inode *Inode) Rename(ctx npo.CtxI, from, to string) error {
	if inode.parent == nil {
		return errors.New("Cannot remove root directory")
	}
	dir := inode.parent
	dir.Lock()
	defer dir.Unlock()

	db.DLPrintf("MEMFS", "%v: Rename %v -> %v\n", dir, from, to)
	ino, err := dir.lookupL(from)
	if err != nil {
		return err
	}
	err = dir.removeL(from)
	if err != nil {
		log.Fatalf("Rename: remove failed %v %v\n", from, err)
	}
	_, err = dir.lookupL(to)
	if err == nil { // i is valid
		// XXX 9p: it is an error to change the name to that
		// of an existing file.
		err = dir.removeL(to)
		if err != nil {
			log.Fatalf("Rename remove failed %v %v\n", to, err)
		}
	}
	err = dir.createL(ino, to)
	if err != nil {
		log.Fatalf("Rename create %v failed %v\n", to, err)
		return err
	}

	return nil
}
