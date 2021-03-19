package fsclnt

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"sync"
	"time"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npclnt"
)

const (
	MAXFD = 20
)

type FdState struct {
	offset np.Toffset
	fid    np.Tfid
}

type FsClient struct {
	mu    sync.Mutex
	fds   []FdState
	fids  map[np.Tfid]*Path
	npc   *npclnt.NpClnt
	mount *Mount
	next  np.Tfid
	uname string
}

func MakeFsClient(uname string) *FsClient {
	fsc := &FsClient{}
	fsc.fds = make([]FdState, 0, MAXFD)
	fsc.fids = make(map[np.Tfid]*Path)
	fsc.mount = makeMount()
	fsc.npc = npclnt.MakeNpClnt(uname)
	fsc.next = 1
	fsc.uname = uname
	rand.Seed(time.Now().UnixNano())
	return fsc
}

func (fsc *FsClient) String() string {
	str := fmt.Sprintf("Fsclnt table:\n")
	str += fmt.Sprintf("fds %v\n", fsc.fds)
	for k, v := range fsc.fids {
		str += fmt.Sprintf("fid %v chan %v\n", k, v)
	}
	return str
}

func (fsc *FsClient) Uname() string {
	return fsc.uname
}

func (fsc *FsClient) npch(fid np.Tfid) *npclnt.NpChan {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()

	return fsc.fids[fid].npch
}

func (fsc *FsClient) path(fid np.Tfid) *Path {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()
	return fsc.fids[fid]
}

func (fsc *FsClient) addFid(fid np.Tfid, path *Path) {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()
	fsc.fids[fid] = path
}

// XXX maybe close channel?
func (fsc *FsClient) freeFidUnlocked(fid np.Tfid) {
	_, ok := fsc.fids[fid]
	if !ok {
		log.Fatalf("%v: freeFid: fid %v unknown\n", fsc.uname, fid)
	}
	delete(fsc.fids, fid)
}

func (fsc *FsClient) freeFid(fid np.Tfid) {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()
	fsc.freeFidUnlocked(fid)
}

func (fsc *FsClient) findfd(nfid np.Tfid) int {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()

	for fd, fdst := range fsc.fds {
		if fdst.fid == np.NoFid {
			fsc.fds[fd].offset = 0
			fsc.fds[fd].fid = nfid
			return fd
		}
	}
	// no free one
	fsc.fds = append(fsc.fds, FdState{0, nfid})
	return len(fsc.fds) - 1
}

func (fsc *FsClient) closefd(fd int) {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()

	fsc.freeFidUnlocked(fsc.fds[fd].fid)
	fsc.fds[fd].fid = np.NoFid
}

func (fsc *FsClient) allocFid() np.Tfid {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()

	fid := fsc.next
	fsc.next += 1
	return fid
}

func (fsc *FsClient) lookup(fd int) (np.Tfid, error) {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()

	if fsc.fds[fd].fid == np.NoFid {
		return np.NoFid, errors.New("Non-existing")
	}
	return fsc.fds[fd].fid, nil
}

func (fsc *FsClient) lookupSt(fd int) (*FdState, error) {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()

	if fd < 0 || fd >= len(fsc.fds) {
		return nil, fmt.Errorf("Too big fd %v", fd)
	}
	if fsc.fds[fd].fid == np.NoFid {
		return nil, fmt.Errorf("Non-existing fd %v", fd)
	}
	return &fsc.fds[fd], nil
}

func (fsc *FsClient) Mount(fid np.Tfid, path string) error {
	_, ok := fsc.fids[fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	db.DLPrintf(fsc.uname, "FSCLNT", "Mount %v at %v %v\n", fid, path, fsc.npch(fid))
	fsc.mount.add(np.Split(path), fid)
	return nil
}

func (fsc *FsClient) Close(fd int) error {
	fid, err := fsc.lookup(fd)
	if err != nil {
		return err
	}
	err = fsc.npch(fid).Clunk(fid)
	if err == nil {
		fsc.closefd(fd)
	}
	return err
}

// XXX if server lives in this process, do something special?  FsClient doesn't
// know about the server currently.
func (fsc *FsClient) attachChannel(fid np.Tfid, server string, p []string) (*Path, error) {
	reply, err := fsc.npc.Attach(fsc.Uname(), server, fsc.Uname(), fid, p)
	if err != nil {
		return nil, err
	}
	ch := fsc.npc.MakeNpChan(fsc.Uname(), server)
	return makePath(ch, p, []np.Tqid{reply.Qid}), nil
}

func (fsc *FsClient) detachChannel(fid np.Tfid) {
	fsc.freeFid(fid)
}

func (fsc *FsClient) Attach(server string, path string) (np.Tfid, error) {
	p := np.Split(path)
	fid := fsc.allocFid()
	ch, err := fsc.attachChannel(fid, server, p)
	if err != nil {
		return np.NoFid, err
	}
	fsc.addFid(fid, ch)
	db.DLPrintf(fsc.uname, "FSCLNT", "Attach -> fid %v %v %v\n", fid, fsc.fids[fid], fsc.fids[fid].npch)
	return fid, nil
}

func (fsc *FsClient) clone(fid np.Tfid) (np.Tfid, error) {
	fid1 := fsc.allocFid()
	_, err := fsc.npch(fid).Walk(fid, fid1, nil)
	if err != nil {
		// XXX free fid
		return np.NoFid, err
	}
	fsc.addFid(fid1, fsc.path(fid).copyPath())
	return fid1, err
}

func (fsc *FsClient) clunkFid(fid np.Tfid) {
	err := fsc.npch(fid).Clunk(fid)
	if err != nil {
		db.DLPrintf(fsc.uname, "FSCLNT", "clunkFid clunk failed %v\n", err)
	}
	fsc.freeFid(fid)
}

func (fsc *FsClient) Create(path string, perm np.Tperm, mode np.Tmode) (int, error) {
	db.DLPrintf(fsc.uname, "FSCLNT", "Create %v perm %v\n", path, perm)
	p := np.Split(path)
	dir := p[0 : len(p)-1]
	base := p[len(p)-1]
	fid, err := fsc.walkMany(dir, true)
	if err != nil {
		return -1, err
	}
	reply, err := fsc.npch(fid).Create(fid, base, perm, mode)
	if err != nil {
		return -1, err
	}
	fsc.path(fid).add(base, reply.Qid)
	fd := fsc.findfd(fid)
	return fd, nil
}

// XXX reduce duplicattion with Create
func (fsc *FsClient) CreateAt(dfd int, name string, perm np.Tperm, mode np.Tmode) (int, error) {
	db.DLPrintf(fsc.uname, "FSCLNT", "CreateAt %v at %v\n", name, dfd)
	fid, err := fsc.lookup(dfd)
	if err != nil {
		return -1, err
	}
	fid1, err := fsc.clone(fid)
	if err != nil {
		return -1, err
	}
	reply, err := fsc.npch(fid1).Create(fid1, name, perm, mode)
	if err != nil {
		return -1, err
	}
	fsc.path(fid1).add(name, reply.Qid)
	fd := fsc.findfd(fid1)
	return fd, nil
}

// XXX The unix 9p client seems to split a rename across directories
// into a create and remove, and only does renames within the same
// directory. For now forget about splitting.
//
// XXX update pathname associated with fid in Channel
func (fsc *FsClient) Rename(old string, new string) error {
	db.DLPrintf(fsc.uname, "FSCLNT", "Rename %v %v\n", old, new)
	opath := np.Split(old)
	npath := np.Split(new)

	if len(opath) != len(npath) {
		return errors.New("Rename must be within same directory")
	}
	for i, n := range opath[:len(opath)-1] {
		if npath[i] != n {
			return errors.New("Rename must be within same directory")
		}
	}
	fid, err := fsc.walkMany(opath, np.EndSlash(old))
	if err != nil {
		return err
	}
	st := &np.Stat{}
	st.Name = npath[len(npath)-1]
	_, err = fsc.npch(fid).Wstat(fid, st)
	return err
}

func (fsc *FsClient) umount(path []string) error {
	db.DLPrintf(fsc.uname, "FSCLNT", "Umount %v\n", path)
	if len(path) < 1 {
		return fmt.Errorf("unmount bad path %v\n", path)
	}
	prefix := make([]string, len(path)-1)
	last := path[len(path)-1:]
	copy(prefix, path[:len(path)-1])
	fid, err := fsc.walkMany(prefix, true)
	if err != nil {
		return fmt.Errorf("Remove walkMany %v error %v\n", prefix, err)
	}
	fid1 := fsc.allocFid()
	_, err = fsc.npch(fid).Walk(fid, fid1, last)
	if err != nil {
		return err
	}
	err = fsc.npch(fid).Remove(fid1)
	if err != nil {
		return err
	}
	fid2, err := fsc.mount.umount(path)
	if err != nil {
		log.Fatalf("del failed\n")
	}
	fsc.detachChannel(fid2)
	return nil
}

// XXX free fid?
func (fsc *FsClient) Remove(name string) error {
	db.DLPrintf(fsc.uname, "FSCLNT", "Remove %v\n", name)
	path := np.Split(name)
	_, rest := fsc.mount.resolve(path)
	if len(rest) == 0 && !np.EndSlash(name) {
		return fsc.umount(path)
	} else {
		fid, err := fsc.walkMany(path, np.EndSlash(name))
		if err != nil {
			return err
		}
		return fsc.npch(fid).Remove(fid)
	}
}

func (fsc *FsClient) Stat(name string) (*np.Stat, error) {
	db.DLPrintf(fsc.uname, "FSCLNT", "Stat %v\n", name)
	path := np.Split(name)
	target, rest := fsc.mount.resolve(path)
	if len(rest) == 0 && !np.EndSlash(name) {
		st := &np.Stat{}
		st.Name = fsc.npch(target).Server()
		return st, nil
	} else {
		fid, err := fsc.walkMany(np.Split(name), np.EndSlash(name))
		if err != nil {
			return nil, err
		}
		reply, err := fsc.npch(fid).Stat(fid)
		if err != nil {
			return nil, err
		}
		return &reply.Stat, nil
	}
}

// XXX clone fid?
func (fsc *FsClient) Readlink(fid np.Tfid) (string, error) {
	_, err := fsc.npch(fid).Open(fid, np.OREAD)
	if err != nil {
		return "", err
	}
	reply, err := fsc.npch(fid).Read(fid, 0, 1024)
	if err != nil {
		return "", err
	}
	// XXX close fid
	return string(reply.Data), nil
}

func (fsc *FsClient) Open(path string, mode np.Tmode) (int, error) {
	db.DLPrintf(fsc.uname, "FSCLNT", "Open %v %v\n", path, mode)
	var fid np.Tfid
	for {
		p := np.Split(path)
		f, err := fsc.walkMany(p, np.EndSlash(path))
		if err == io.EOF {
			fid2, e := fsc.mount.umount(p)
			if e != nil {
				return -1, err
			}
			fsc.detachChannel(fid2)
			continue
		}
		if err != nil {
			return -1, err
		}
		fid = f
		break
	}
	_, err := fsc.npch(fid).Open(fid, mode)
	if err != nil {
		return -1, err
	}
	// XXX check reply.Qid?
	fd := fsc.findfd(fid)
	return fd, nil

}

func (fsc *FsClient) Read(fd int, cnt np.Tsize) ([]byte, error) {
	fdst, err := fsc.lookupSt(fd)
	if err != nil {
		return nil, err
	}
	reply, err := fsc.npch(fdst.fid).Read(fdst.fid, fdst.offset, cnt)
	if err != nil {
		return nil, err
	}
	fdst.offset += np.Toffset(len(reply.Data))
	return reply.Data, err
}

func (fsc *FsClient) Write(fd int, data []byte) (np.Tsize, error) {
	fdst, err := fsc.lookupSt(fd)
	if err != nil {
		return 0, err
	}
	reply, err := fsc.npch(fdst.fid).Write(fdst.fid, fdst.offset, data)
	if err != nil {
		return 0, err
	}
	fdst.offset += np.Toffset(reply.Count)
	return reply.Count, err
}
