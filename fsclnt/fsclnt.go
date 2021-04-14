package fsclnt

import (
	"errors"
	"fmt"
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

type Watch func(string)

type FdState struct {
	offset np.Toffset
	fid    np.Tfid
	mode   np.Tmode
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
	fsc.npc = npclnt.MakeNpClnt()
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

func (fsc *FsClient) findfd(nfid np.Tfid, m np.Tmode) int {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()

	for fd, fdst := range fsc.fds {
		if fdst.fid == np.NoFid {
			fsc.fds[fd].offset = 0
			fsc.fds[fd].fid = nfid
			fsc.fds[fd].mode = m
			return fd
		}
	}
	// no free one
	fsc.fds = append(fsc.fds, FdState{0, nfid, m})
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

func (fsc *FsClient) lookupStL(fd int) (*FdState, error) {

	if fd < 0 || fd >= len(fsc.fds) {
		return nil, fmt.Errorf("Too big fd %v", fd)
	}
	if fsc.fds[fd].fid == np.NoFid {
		return nil, fmt.Errorf("Non-existing fd %v", fd)
	}
	return &fsc.fds[fd], nil
}

func (fsc *FsClient) lookupSt(fd int) (*FdState, error) {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()
	return fsc.lookupStL(fd)
}

// Wrote this in the CAS style, unsure if it's overkill
func (fsc *FsClient) stOffsetCAS(fd int, oldOff np.Toffset, newOff np.Toffset) (bool, error) {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()

	if fd < 0 || fd >= len(fsc.fds) {
		return false, fmt.Errorf("Too big fd %v", fd)
	}
	if fsc.fds[fd].fid == np.NoFid {
		return false, fmt.Errorf("Non-existing fd %v", fd)
	}
	fdst := &fsc.fds[fd]
	if fdst.offset == oldOff {
		fdst.offset = newOff
		return true, nil
	}
	return false, nil
}

func (fsc *FsClient) Mount(fid np.Tfid, path string) error {
	fsc.mu.Lock()
	_, ok := fsc.fids[fid]
	fsc.mu.Unlock()
	if !ok {
		return errors.New("Unknown fid")
	}
	db.DLPrintf("FSCLNT", "Mount %v at %v %v\n", fid, path, fsc.npch(fid))
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
	reply, err := fsc.npc.Attach(server, fsc.Uname(), fid, p)
	if err != nil {
		return nil, err
	}
	ch := fsc.npc.MakeNpChan(server)
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

	fsc.mu.Lock()
	defer fsc.mu.Unlock()

	db.DLPrintf("FSCLNT", "Attach -> fid %v %v %v\n", fid, fsc.fids[fid], fsc.fids[fid].npch)
	return fid, nil
}

func (fsc *FsClient) clone(fid np.Tfid) (np.Tfid, error) {
	db.DLPrintf("FSCLNT", "clone: %v %v\n", fid, fsc.path(fid))
	fid2 := fsc.path(fid)
	if fid2 == nil {
		return np.NoFid, errors.New("Unknown file")
	}
	path := fid2.copyPath()
	fid1 := fsc.allocFid()
	_, err := fid2.npch.Walk(fid, fid1, nil)
	if err != nil {
		// XXX free fid
		return np.NoFid, err
	}
	db.DLPrintf("FSCLNT", "clone: %v %v -> %v\n", fid, fsc.path(fid), fid1)
	fsc.addFid(fid1, path)
	return fid1, err
}

func (fsc *FsClient) clunkFid(fid np.Tfid) {
	err := fsc.npch(fid).Clunk(fid)
	if err != nil {
		db.DLPrintf("FSCLNT", "clunkFid clunk failed %v\n", err)
	}
	fsc.freeFid(fid)
}

func (fsc *FsClient) Create(path string, perm np.Tperm, mode np.Tmode) (int, error) {
	db.DLPrintf("FSCLNT", "Create %v perm %v\n", path, perm)
	p := np.Split(path)
	dir := p[0 : len(p)-1]
	base := p[len(p)-1]
	fid, err := fsc.walkMany(dir, true, nil)
	if err != nil {
		return -1, err
	}
	reply, err := fsc.npch(fid).Create(fid, base, perm, mode)
	if err != nil {
		return -1, err
	}
	fsc.path(fid).add(base, reply.Qid)
	fd := fsc.findfd(fid, mode)
	return fd, nil
}

// XXX The unix 9p client seems to split a rename across directories
// into a create and remove, and only does renames within the same
// directory. For now forget about splitting.
//
// XXX update pathname associated with fid in Channel
func (fsc *FsClient) Rename(old string, new string) error {
	db.DLPrintf("FSCLNT", "Rename %v %v\n", old, new)
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
	fid, err := fsc.walkMany(opath, np.EndSlash(old), nil)
	if err != nil {
		return err
	}
	st := &np.Stat{}
	st.Name = npath[len(npath)-1]
	_, err = fsc.npch(fid).Wstat(fid, st)
	return err
}

func (fsc *FsClient) Umount(path []string) error {
	db.DLPrintf("FSCLNT", "Umount %v\n", path)
	fid2, err := fsc.mount.umount(path)
	if err != nil {
		return err
	}
	fsc.detachChannel(fid2)
	return nil
}

func (fsc *FsClient) removeMount(path []string) error {
	if len(path) < 1 {
		return fmt.Errorf("unmount bad path %v\n", path)
	}
	prefix := make([]string, len(path)-1)
	last := path[len(path)-1:]
	copy(prefix, path[:len(path)-1])
	fid, err := fsc.walkMany(prefix, true, nil)
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
	return fsc.Umount(path)
}

// XXX free fid?
func (fsc *FsClient) Remove(name string) error {
	db.DLPrintf("FSCLNT", "Remove %v\n", name)
	path := np.Split(name)
	_, rest := fsc.mount.resolve(path)
	if len(rest) == 0 && !np.EndSlash(name) { // mount point
		return fsc.removeMount(path)
	} else {
		fid, err := fsc.walkMany(path, np.EndSlash(name), nil)
		if err != nil {
			return err
		}
		return fsc.npch(fid).Remove(fid)
	}
}

func (fsc *FsClient) Stat(name string) (*np.Stat, error) {
	db.DLPrintf("FSCLNT", "Stat %v\n", name)
	path := np.Split(name)
	target, rest := fsc.mount.resolve(path)
	if len(rest) == 0 && !np.EndSlash(name) {
		st := &np.Stat{}
		st.Name = fsc.npch(target).Server()
		return st, nil
	} else {
		fid, err := fsc.walkMany(np.Split(name), np.EndSlash(name), nil)
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
	db.DLPrintf("FSCLNT", "ReadLink %v\n", fid)
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

func (fsc *FsClient) OpenWatch(path string, mode np.Tmode, f Watch) (int, error) {
	db.DLPrintf("FSCLNT", "Open %v %v\n", path, mode)
	p := np.Split(path)
	fid, err := fsc.WalkManyUmount(p, np.EndSlash(path), f)
	if err != nil {
		return -1, err
	}
	_, err = fsc.npch(fid).Open(fid, mode)
	if err != nil {
		return -1, err
	}
	// XXX check reply.Qid?
	fd := fsc.findfd(fid, mode)
	return fd, nil
}

func (fsc *FsClient) Open(path string, mode np.Tmode) (int, error) {
	return fsc.OpenWatch(path, mode, nil)
}

func (fsc *FsClient) Read(fd int, cnt np.Tsize) ([]byte, error) {
	db.DLPrintf("FSCLNT", "Read %v %v\n", fd, cnt)
	fsc.mu.Lock()
	fdst, err := fsc.lookupStL(fd)
	if err != nil {
		fsc.mu.Unlock()
		return nil, err
	}
	p := fsc.fids[fdst.fid]
	version := p.lastqid().Version
	v := fdst.mode&np.OVERSION == np.OVERSION
	fsc.mu.Unlock()
	var reply *np.Rread
	if v {
		reply, err = fsc.npch(fdst.fid).ReadV(fdst.fid, fdst.offset, cnt, version)
	} else {
		reply, err = fsc.npch(fdst.fid).Read(fdst.fid, fdst.offset, cnt)
	}
	if err != nil {
		return nil, err
	}

	// Can't reuse the fdst without looking it up again, since the fdst may
	// have changed and now point to the wrong location. So instead, try and CAS
	// the new offset
	for ok, err := fsc.stOffsetCAS(fd, fdst.offset, fdst.offset+np.Toffset(len(reply.Data))); !ok; {
		if err != nil {
			return nil, err
		}
		fdst, _ = fsc.lookupSt(fd)
	}
	db.DLPrintf("FSCLNT", "Read -> %v %v\n", reply, err)
	return reply.Data, err
}

func (fsc *FsClient) Write(fd int, data []byte) (np.Tsize, error) {
	db.DLPrintf("FSCLNT", "Write %v %v\n", fd, len(data))
	fsc.mu.Lock()
	fdst, err := fsc.lookupStL(fd)
	if err != nil {
		fsc.mu.Unlock()
		return 0, err
	}
	p := fsc.fids[fdst.fid]
	version := p.lastqid().Version
	v := fdst.mode&np.OVERSION == np.OVERSION
	fsc.mu.Unlock()
	var reply *np.Rwrite
	if v {
		reply, err = fsc.npch(fdst.fid).WriteV(fdst.fid, fdst.offset, data, version)
	} else {
		reply, err = fsc.npch(fdst.fid).Write(fdst.fid, fdst.offset, data)
	}

	if err != nil {
		return 0, err
	}

	// Can't reuse the fdst without looking it up again, since the fdst may
	// have changed and now point to the wrong location. So instead, try and CAS
	// the new offset
	for ok, err := fsc.stOffsetCAS(fd, fdst.offset, fdst.offset+np.Toffset(reply.Count)); !ok; {
		if err != nil {
			return 0, err
		}
		fdst, _ = fsc.lookupSt(fd)
	}

	return reply.Count, err
}

func (fsc *FsClient) Lseek(fd int, off np.Toffset) error {
	db.DLPrintf("FSCLNT", "Lseek %v %v\n", fd, off)
	fsc.mu.Lock()
	defer fsc.mu.Unlock()

	fdst, err := fsc.lookupStL(fd)
	if err != nil {
		return err
	}
	fdst.offset = off
	return nil
}
