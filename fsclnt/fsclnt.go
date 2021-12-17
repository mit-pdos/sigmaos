package fsclnt

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/lease"
	np "ulambda/ninep"
	"ulambda/protclnt"
)

const (
	MAXFD = 20
)

type Watch func(string, error)

type FdState struct {
	offset np.Toffset
	fid    np.Tfid
	mode   np.Tmode
}

type FsClient struct {
	sync.Mutex
	fds   []FdState
	fids  map[np.Tfid]*Path
	pc    *protclnt.Clnt
	mount *Mount
	next  np.Tfid
	uname string
	lease *lease.Lease
}

func MakeFsClient(uname string) *FsClient {
	fsc := &FsClient{}
	fsc.fds = make([]FdState, 0, MAXFD)
	fsc.fids = make(map[np.Tfid]*Path)
	fsc.mount = makeMount()
	fsc.pc = protclnt.MakeClnt()
	fsc.uname = uname
	fsc.next = 0
	return fsc
}

func (fsc *FsClient) ReadSeqNo() np.Tseqno {
	return fsc.pc.ReadSeqNo()
}

func (fsc *FsClient) Exit() {
	fsc.mount.exit()
	fsc.pc.Exit()
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

func (fsc *FsClient) clnt(fid np.Tfid) *protclnt.ProtClnt {
	fsc.Lock()
	defer fsc.Unlock()

	return fsc.fids[fid].pc
}

func (fsc *FsClient) path(fid np.Tfid) *Path {
	fsc.Lock()
	defer fsc.Unlock()
	return fsc.fids[fid]
}

func (fsc *FsClient) addFid(fid np.Tfid, path *Path) {
	fsc.Lock()
	defer fsc.Unlock()
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
	fsc.Lock()
	defer fsc.Unlock()
	fsc.freeFidUnlocked(fid)
}

func (fsc *FsClient) findfd(nfid np.Tfid, m np.Tmode) int {
	fsc.Lock()
	defer fsc.Unlock()

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
	fsc.Lock()
	defer fsc.Unlock()

	fsc.freeFidUnlocked(fsc.fds[fd].fid)
	fsc.fds[fd].fid = np.NoFid
}

func (fsc *FsClient) allocFid() np.Tfid {
	fsc.Lock()
	defer fsc.Unlock()

	fid := fsc.next
	fsc.next += 1
	return fid
}

func (fsc *FsClient) lookup(fd int) (np.Tfid, error) {
	fsc.Lock()
	defer fsc.Unlock()

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
	fsc.Lock()
	defer fsc.Unlock()
	return fsc.lookupStL(fd)
}

// Wrote this in the CAS style, unsure if it's overkill
func (fsc *FsClient) stOffsetCAS(fd int, oldOff np.Toffset, newOff np.Toffset) (bool, error) {
	fsc.Lock()
	defer fsc.Unlock()

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

// Simulate network partition to server that exports path
func (fsc *FsClient) Disconnect(path string) error {
	p := np.Split(path)
	fid, _ := fsc.mount.resolve(p)
	if fid == np.NoFid {
		db.DLPrintf("FSCLNT", "Disconnect: resolve  unknown fid\n")
		if fsc.mount.hasExited() {
			return io.EOF
		}
		return errors.New("file not found")
	}
	clnt := fsc.clnt(fid)
	clnt.Disconnect()
	return nil
}

func (fsc *FsClient) Mount(fid np.Tfid, path string) error {
	fsc.Lock()
	_, ok := fsc.fids[fid]
	fsc.Unlock()
	if !ok {
		return errors.New("Unknown fid")
	}
	db.DLPrintf("FSCLNT", "Mount %v at %v %v\n", fid, path, fsc.clnt(fid))
	fsc.mount.add(np.Split(path), fid)
	fsc.Lock()
	lease := fsc.lease
	fsc.Unlock()
	if lease != nil {
		fsc.pc.RegisterLease(lease)
	}
	return nil
}

func (fsc *FsClient) Close(fd int) error {
	fid, err := fsc.lookup(fd)
	if err != nil {
		return err
	}
	err = fsc.clnt(fid).Clunk(fid)
	if err == nil {
		fsc.closefd(fd)
	}
	return err
}

func (fsc *FsClient) attachChannel(fid np.Tfid, server []string, p []string, tree []string) (*Path, error) {
	reply, err := fsc.pc.Attach(server, fsc.Uname(), fid, tree)
	if err != nil {
		return nil, err
	}
	ch := fsc.pc.MakeProtClnt(server)
	return makePath(ch, p, []np.Tqid{reply.Qid}), nil
}

func (fsc *FsClient) detachChannel(fid np.Tfid) {
	fsc.freeFid(fid)
}

// XXX a version that finds server based on pathname?
func (fsc *FsClient) AttachReplicas(server []string, path, tree string) (np.Tfid, error) {
	p := np.Split(path)
	fid := fsc.allocFid()
	ch, err := fsc.attachChannel(fid, server, p, np.Split(tree))
	if err != nil {
		return np.NoFid, err
	}
	fsc.addFid(fid, ch)

	fsc.Lock()
	defer fsc.Unlock()

	db.DLPrintf("FSCLNT", "Attach -> fid %v %v %v\n", fid, fsc.fids[fid], fsc.fids[fid].pc)
	return fid, nil
}

func (fsc *FsClient) Attach(server, path, tree string) (np.Tfid, error) {
	return fsc.AttachReplicas([]string{server}, path, tree)
}

func (fsc *FsClient) RegisterLease(l *lease.Lease) error {
	fsc.Lock()
	if fsc.lease != nil {
		fsc.Unlock()
		return fmt.Errorf("%v already leased\n", l.Fn)
	}
	fsc.lease = l
	fsc.Unlock()
	return fsc.pc.RegisterLease(l)
}

func (fsc *FsClient) DeregisterLease(path string) error {
	fsc.Lock()
	if fsc.lease == nil {
		fsc.Unlock()
		return fmt.Errorf("%v not leased\n", path)
	}
	fsc.lease = nil
	fsc.Unlock()
	return fsc.pc.DeregisterLease(np.Split(path))
}

func (fsc *FsClient) clone(fid np.Tfid) (np.Tfid, error) {
	db.DLPrintf("FSCLNT", "clone: %v %v\n", fid, fsc.path(fid))
	fid2 := fsc.path(fid)
	if fid2 == nil {
		return np.NoFid, errors.New("unknown file")
	}
	path := fid2.copyPath()
	fid1 := fsc.allocFid()
	_, err := fid2.pc.Walk(fid, fid1, nil)
	if err != nil {
		// XXX free fid
		return np.NoFid, err
	}
	db.DLPrintf("FSCLNT", "clone: %v %v -> %v\n", fid, fsc.path(fid), fid1)
	fsc.addFid(fid1, path)
	return fid1, err
}

func (fsc *FsClient) clunkFid(fid np.Tfid) {
	err := fsc.clnt(fid).Clunk(fid)
	if err != nil {
		db.DLPrintf("FSCLNT", "clunkFid clunk failed %v\n", err)
	}
	fsc.freeFid(fid)
}

func (fsc *FsClient) Create(path string, perm np.Tperm, mode np.Tmode) (int, error) {
	db.DLPrintf("FSCLNT", "Create %v perm %v\n", path, perm)
	p := np.Split(path)
	dir := np.Dir(p)
	base := np.Base(p)
	fid, err := fsc.walkMany(dir, true, nil)
	if err != nil {
		return -1, err
	}
	reply, err := fsc.clnt(fid).Create(fid, base, perm, mode)
	if err != nil {
		return -1, err
	}
	fsc.path(fid).add(base, reply.Qid)
	fd := fsc.findfd(fid, mode)
	return fd, nil
}

// Rename using renameat() for across directories or using wstat()
// for within a directory.
func (fsc *FsClient) Rename(old string, new string) error {
	db.DLPrintf("FSCLNT", "Rename %v %v\n", old, new)
	opath := np.Split(old)
	npath := np.Split(new)

	if len(opath) != len(npath) {
		return fsc.renameat(old, new)
	}
	for i, n := range opath[:len(opath)-1] {
		if npath[i] != n {
			return fsc.renameat(old, new)
		}
	}
	fid, err := fsc.walkMany(opath, np.EndSlash(old), nil)
	if err != nil {
		return err
	}
	st := &np.Stat{}
	st.Name = npath[len(npath)-1]
	_, err = fsc.clnt(fid).Wstat(fid, st)
	return err
}

// Rename across directories of a single server using Renameat
func (fsc *FsClient) renameat(old, new string) error {
	db.DLPrintf("FSCLNT", "Renameat %v %v\n", old, new)
	opath := np.Split(old)
	npath := np.Split(new)
	o := opath[len(opath)-1]
	n := npath[len(npath)-1]
	fid, err := fsc.walkMany(opath[:len(opath)-1], np.EndSlash(old), nil)
	if err != nil {
		return err
	}
	defer fsc.clunkFid(fid)
	fid1, err := fsc.walkMany(npath[:len(npath)-1], np.EndSlash(old), nil)
	if err != nil {
		return err
	}
	defer fsc.clunkFid(fid1)
	if fsc.clnt(fid) != fsc.clnt(fid1) {
		return fmt.Errorf("Renameat: files not at same server")
	}
	_, err = fsc.clnt(fid).Renameat(fid, o, fid1, n)
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

// XXX free fid?
func (fsc *FsClient) Remove(name string) error {
	db.DLPrintf("FSCLNT", "Remove %v\n", name)
	path := np.Split(name)
	fid, rest := fsc.mount.resolve(path)
	if fid == np.NoFid {
		db.DLPrintf("FSCLNT", "Remove: resolve unknown fid\n")
		if fsc.mount.hasExited() {
			return io.EOF
		}
		return errors.New("file not found")
	}
	// Optimistcally remove obj without doing a pathname
	// walk; this may fail if rest contains an automount
	// symlink.
	err := fsc.clnt(fid).RemoveFile(fid, rest)
	if err != nil {
		// There must have been a symlink in rest
		// force automounting, but only on dir error
		if strings.HasPrefix(err.Error(), "dir not found") {
			fid, err = fsc.WalkManyUmount(path, np.EndSlash(name), nil)
			if err != nil {
				return err
			}
			err = fsc.clnt(fid).Remove(fid)
		}
	}
	return err
}

func (fsc *FsClient) Stat(name string) (*np.Stat, error) {
	db.DLPrintf("FSCLNT", "Stat %v\n", name)
	path := np.Split(name)
	target, rest := fsc.mount.resolve(path)
	if len(rest) == 0 && !np.EndSlash(name) {
		st := &np.Stat{}
		st.Name = strings.Join(fsc.clnt(target).Server(), ",")
		return st, nil
	} else {
		fid, err := fsc.walkMany(np.Split(name), np.EndSlash(name), nil)
		if err != nil {
			return nil, err
		}
		reply, err := fsc.clnt(fid).Stat(fid)
		if err != nil {
			return nil, err
		}
		return &reply.Stat, nil
	}
}

func (fsc *FsClient) Readlink(fid np.Tfid) (string, error) {
	db.DLPrintf("FSCLNT", "ReadLink %v\n", fid)
	clnt := fsc.clnt(fid)
	return fsc.readlink(clnt, fid)
}

// XXX clone fid?
func (fsc *FsClient) readlink(pc *protclnt.ProtClnt, fid np.Tfid) (string, error) {
	db.DLPrintf("FSCLNT", "readLink %v\n", fid)
	_, err := pc.Open(fid, np.OREAD)
	if err != nil {
		return "", err
	}
	reply, err := pc.Read(fid, 0, 1024)
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
	_, err = fsc.clnt(fid).Open(fid, mode)
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

func (fsc *FsClient) SetDirWatch(path string, f Watch) error {
	db.DLPrintf("FSCLNT", "SetDirWatch %v\n", path)
	p := np.Split(path)
	fid, err := fsc.WalkManyUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return err
	}
	go func() {
		version := fsc.path(fid).lastqid().Version
		err := fsc.clnt(fid).Watch(fid, nil, version)
		db.DLPrintf("FSCLNT", "SetDirWatch: Watch returns %v %v\n", path, err)
		f(path, err)
	}()
	return nil
}

func (fsc *FsClient) SetRemoveWatch(path string, f Watch) error {
	p := np.Split(path)
	fid, err := fsc.WalkManyUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return err
	}
	if f == nil {
		return os.ErrInvalid
	}
	go func() {
		version := fsc.path(fid).lastqid().Version
		err := fsc.clnt(fid).Watch(fid, nil, version)
		db.DLPrintf("FSCLNT", "SetRemoveWatch: Watch returns %v %v\n", path, err)
		f(path, err)
	}()
	return nil
}

func (fsc *FsClient) Read(fd int, cnt np.Tsize) ([]byte, error) {
	db.DLPrintf("FSCLNT", "Read %v %v\n", fd, cnt)
	fsc.Lock()
	fdst, err := fsc.lookupStL(fd)
	if err != nil {
		fsc.Unlock()
		return nil, err
	}
	fsc.Unlock()
	var reply *np.Rread
	reply, err = fsc.clnt(fdst.fid).Read(fdst.fid, fdst.offset, cnt)
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
	db.DLPrintf("FSCLNT", "Read %v -> %v %v\n", fd, reply, err)
	return reply.Data, err
}

func (fsc *FsClient) Write(fd int, data []byte) (np.Tsize, error) {
	db.DLPrintf("FSCLNT", "Write %v %v\n", fd, len(data))
	fsc.Lock()
	fdst, err := fsc.lookupStL(fd)
	if err != nil {
		fsc.Unlock()
		return 0, err
	}
	fsc.Unlock()
	var reply *np.Rwrite
	reply, err = fsc.clnt(fdst.fid).Write(fdst.fid, fdst.offset, data)

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
	fsc.Lock()
	defer fsc.Unlock()

	fdst, err := fsc.lookupStL(fd)
	if err != nil {
		return err
	}
	fdst.offset = off
	return nil
}

func (fsc *FsClient) GetFile(path string, mode np.Tmode) ([]byte, np.TQversion, error) {
	db.DLPrintf("FSCLNT", "GetFile %v %v\n", path, mode)
	p := np.Split(path)
	fid, rest := fsc.mount.resolve(p)
	if fid == np.NoFid {
		db.DLPrintf("FSCLNT", "GetFile: mount -> unknown fid\n")
		if fsc.mount.hasExited() {
			return nil, np.NoV, io.EOF
		}
		return nil, np.NoV, errors.New("file not found")

	}
	reply, err := fsc.clnt(fid).GetFile(fid, rest, mode, 0, 0)
	if err != nil {
		// force automounting, but only on dir error
		if strings.HasPrefix(err.Error(), "dir not found") {
			fid, err = fsc.WalkManyUmount(p, np.EndSlash(path), nil)
			if err != nil {
				return nil, np.NoV, err
			}
			reply, err = fsc.clnt(fid).GetFile(fid, []string{}, mode, 0, 0)
			if err != nil {
				return nil, np.NoV, err
			}
		} else {
			return nil, np.NoV, err
		}
	}
	return reply.Data, reply.Version, err
}

// Create or write file (if perm = 0).
func (fsc *FsClient) SetFile(path string, mode np.Tmode, perm np.Tperm, version np.TQversion, data []byte) (np.Tsize, error) {
	db.DLPrintf("FSCLNT", "SetFile %v %v\n", path, mode)
	p := np.Split(path)
	dir := np.Dir(p)
	base := []string{np.Base(p)}
	fid, rest := fsc.mount.resolve(p)
	if fid == np.NoFid {
		db.DLPrintf("FSCLNT", "SetFile: mount -> unknown fid\n")
		if fsc.mount.hasExited() {
			return 0, io.EOF
		}
		return 0, errors.New("file not found")

	}
	reply, err := fsc.clnt(fid).SetFile(fid, rest, mode, perm, 0, version, data)
	if err != nil {
		// force automounting, but only on dir error
		if strings.HasPrefix(err.Error(), "dir not found") {
			if perm == 0 {
				dir = p
				base = []string{"."}
			}
			fid, err = fsc.WalkManyUmount(dir, true, nil)
			if err != nil {
				return 0, err
			}
			reply, err = fsc.clnt(fid).SetFile(fid, base, mode, perm, 0, version, data)
			if err != nil {
				return 0, err
			}
		} else {
			return 0, err
		}
	}
	return reply.Count, err
}

func (fsc *FsClient) ShutdownFs(name string) error {
	db.DLPrintf("FSCLNT", "ShutdownFs %v\n", name)
	path := np.Split(name)
	fid, err := fsc.walkMany(path, true, nil)
	if err != nil {
		return fmt.Errorf("ShutdownFs walkMany %v error %v\n", path, err)
	}
	err = fsc.clnt(fid).RemoveFile(fid, []string{".exit"})
	if err != nil {
		return err
	}
	return fsc.Umount(path)
}
