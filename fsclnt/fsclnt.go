package fsclnt

import (
	"fmt"
	"log"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/fences"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/protclnt"
)

//
// Sigma file system API.  A sigma proc typically has one fsclnt
// through which it interacts with all the file servers in a sigma
// deployment.
//

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
	mnt   *Mount
	next  np.Tfid
	uname string
	fm    *fences.FenceTable
}

func MakeFsClient(uname string) *FsClient {
	fsc := &FsClient{}
	fsc.fds = make([]FdState, 0, MAXFD)
	fsc.fids = make(map[np.Tfid]*Path)
	fsc.mnt = makeMount()
	fsc.pc = protclnt.MakeClnt()
	fsc.uname = uname
	fsc.next = 0
	fsc.fm = fences.MakeFenceTable()
	return fsc
}

func (fsc *FsClient) ReadSeqNo() np.Tseqno {
	return fsc.pc.ReadSeqNo()
}

func (fsc *FsClient) Exit() {
	fsc.mnt.exit()
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
		log.Fatalf("FATAL %v: freeFid: fid %v unknown\n", fsc.uname, fid)
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
		return np.NoFid, np.MkErr(np.TErrUnknownfid, "lookup")
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
	fid, _ := fsc.mnt.resolve(p)
	if fid == np.NoFid {
		db.DLPrintf("FSCLNT", "Disconnect: resolve  unknown fid\n")
		if fsc.mnt.hasExited() {
			return np.MkErr(np.TErrEOF, path)
		}
		return np.MkErr(np.TErrNotfound, fmt.Sprintf("no mount for %v\n", path))
	}
	clnt := fsc.clnt(fid)
	clnt.Disconnect()
	return nil
}

func (fsc *FsClient) mount(fid np.Tfid, path string) *np.Err {
	fsc.Lock()
	_, ok := fsc.fids[fid]
	fsc.Unlock()
	if !ok {
		return np.MkErr(np.TErrUnknownfid, "mount")
	}
	db.DLPrintf("FSCLNT", "Mount %v at %v %v\n", fid, path, fsc.clnt(fid))
	error := fsc.mnt.add(np.Split(path), fid)
	if error != nil {
		log.Printf("%v: mount %v err %v\n", proc.GetProgram(), path, error)
	}

	for _, f := range fsc.fm.Fences() {
		if err := fsc.pc.RegisterFence(f, true); err != nil {
			return err
		}
	}
	return nil
}

func (fsc *FsClient) Mount(fid np.Tfid, path string) error {
	if err := fsc.mount(fid, path); err != nil {
		return err
	}
	return nil
}

func (fsc *FsClient) Close(fd int) error {
	fid, error := fsc.lookup(fd)
	if error != nil {
		return error
	}
	err := fsc.clnt(fid).Clunk(fid)
	if err != nil {
		return err
	}
	fsc.closefd(fd)
	return nil
}

func (fsc *FsClient) attachChannel(fid np.Tfid, server []string, p []string, tree []string) (*Path, *np.Err) {
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
func (fsc *FsClient) attachReplicas(server []string, path, tree string) (np.Tfid, *np.Err) {
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

func (fsc *FsClient) attach(server, path, tree string) (np.Tfid, *np.Err) {
	return fsc.attachReplicas([]string{server}, path, tree)
}

func (fsc *FsClient) AttachReplicas(server []string, path, tree string) (np.Tfid, error) {
	if fid, err := fsc.attachReplicas(server, path, tree); err != nil {
		return np.NoFid, err
	} else {
		return fid, nil
	}
}

func (fsc *FsClient) MakeFence(path string, mode np.Tmode) (np.Tfence, error) {
	p := np.Split(path)
	fid, err := fsc.walkManyUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return np.Tfence{}, err
	}
	_, err = fsc.clnt(fid).Open(fid, mode)
	if err != nil {
		return np.Tfence{}, err
	}
	defer fsc.clunkFid(fid)
	reply, err := fsc.clnt(fid).MkFence(fid)
	if err != nil {
		log.Printf("%v: MkFence %v err %v\n", proc.GetProgram(), fid, err)
		return np.Tfence{}, err
	}
	return reply.Fence, nil
}

// XXX not thread safe
func (fsc *FsClient) RegisterFence(f np.Tfence) error {
	if ok := fsc.fm.Present(f.FenceId); ok {
		return np.MkErr(np.TErrExists, f.FenceId)
	}
	if err := fsc.pc.RegisterFence(f, true); err != nil {
		return err
	}
	fsc.fm.Insert(f)
	return nil
}

func (fsc *FsClient) UpdateFence(f np.Tfence) error {
	if ok := fsc.fm.Present(f.FenceId); !ok {
		log.Printf("%v: update fence %v not present\n", proc.GetProgram(), f)
		return np.MkErr(np.TErrUnknownFence, f.FenceId)
	}
	if err := fsc.pc.RegisterFence(f, false); err != nil {
		return err
	}
	return nil
}

func (fsc *FsClient) DeregisterFence(f np.Tfence) error {
	if err := fsc.fm.Del(f.FenceId); err != nil {
		log.Printf("%v: dereg %v err %v\n", proc.GetProgram(), f, err)
		return err
	}
	if err := fsc.pc.DeregisterFence(f); err != nil {
		return err
	}
	return nil
}

func (fsc *FsClient) DeregisterFences() error {
	for _, f := range fsc.fm.Fences() {
		fsc.DeregisterFence(f)
	}
	return nil
}

func (fsc *FsClient) RmFence(f np.Tfence) error {
	if ok := fsc.fm.Present(f.FenceId); !ok {
		return np.MkErr(np.TErrUnknownFence, f.FenceId)
	}
	if err := fsc.pc.RmFence(f); err != nil {
		return err
	}
	return nil
}

func (fsc *FsClient) clone(fid np.Tfid) (np.Tfid, *np.Err) {
	db.DLPrintf("FSCLNT", "clone: %v %v\n", fid, fsc.path(fid))
	fid2 := fsc.path(fid)
	if fid2 == nil {
		return np.NoFid, np.MkErr(np.TErrNotfound, "clone")
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
	return fid1, nil
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
		if err := fsc.renameat(old, new); err != nil {
			return err
		}
		return nil
	}
	for i, n := range opath[:len(opath)-1] {
		if npath[i] != n {
			if err := fsc.renameat(old, new); err != nil {
				return err
			}
			return nil
		}
	}
	fid, err := fsc.walkMany(opath, np.EndSlash(old), nil)
	if err != nil {
		return err
	}
	st := &np.Stat{}
	st.Name = npath[len(npath)-1]
	_, err = fsc.clnt(fid).Wstat(fid, st)
	if err != nil {
		return err
	}
	return nil
}

// Rename across directories of a single server using Renameat
func (fsc *FsClient) renameat(old, new string) *np.Err {
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
		return np.MkErr(np.TErrInval, "paths at different servers")
	}
	if _, err = fsc.clnt(fid).Renameat(fid, o, fid1, n); err != nil {
		return err
	}
	return nil
}

func (fsc *FsClient) Umount(path []string) error {
	db.DLPrintf("FSCLNT", "Umount %v\n", path)
	fid2, err := fsc.mnt.umount(path)
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
	fid, rest := fsc.mnt.resolve(path)
	if fid == np.NoFid {
		db.DLPrintf("FSCLNT", "Remove: resolve unknown fid\n")
		if fsc.mnt.hasExited() {
			return np.MkErr(np.TErrEOF, path)
		}
		return np.MkErr(np.TErrNotfound, path)
	}
	// Optimistcally remove obj without doing a pathname
	// walk; this may fail if rest contains an automount
	// symlink.
	err := fsc.clnt(fid).RemoveFile(fid, rest)
	if err != nil {
		// If server could only partially resolve name, it may
		// have been because name contained a symbolic link
		// for a server; retry with resolving name.
		if np.IsDirNotFound(err) {
			fid, err = fsc.walkManyUmount(path, np.EndSlash(name), nil)
			if err != nil {
				return err
			}
			err = fsc.clnt(fid).Remove(fid)
		}
	}
	if err != nil {
		return err
	}
	return nil
}

func (fsc *FsClient) Stat(name string) (*np.Stat, error) {
	db.DLPrintf("FSCLNT", "Stat %v\n", name)
	path := np.Split(name)
	target, rest := fsc.mnt.resolve(path)
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

// XXX clone fid?
func (fsc *FsClient) readlink(pc *protclnt.ProtClnt, fid np.Tfid) (string, *np.Err) {
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

func (fsc *FsClient) OpenWatch(path string, mode np.Tmode, w Watch) (int, error) {
	db.DLPrintf("FSCLNT", "Open %v %v\n", path, mode)
	p := np.Split(path)
	fid, err := fsc.walkManyUmount(p, np.EndSlash(path), w)
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

func (fsc *FsClient) SetDirWatch(path string, w Watch) error {
	db.DLPrintf("FSCLNT", "SetDirWatch %v\n", path)
	p := np.Split(path)
	fid, err := fsc.walkManyUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return err
	}
	go func() {
		version := fsc.path(fid).lastqid().Version
		err := fsc.clnt(fid).Watch(fid, nil, version)
		db.DLPrintf("FSCLNT", "SetDirWatch: Watch returns %v %v\n", path, err)
		w(path, err)
	}()
	return nil
}

func (fsc *FsClient) SetRemoveWatch(path string, w Watch) error {
	p := np.Split(path)
	fid, err := fsc.walkManyUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return err
	}
	if w == nil {
		return np.MkErr(np.TErrInval, "watch")
	}
	go func() {
		version := fsc.path(fid).lastqid().Version
		if err := fsc.clnt(fid).Watch(fid, nil, version); err != nil {
			db.DLPrintf("FSCLNT", "SetRemoveWatch: Watch returns %v %v\n", path, err)
			w(path, err)
		} else {
			w(path, nil)
		}

	}()
	return nil
}

func (fsc *FsClient) Read(fd int, cnt np.Tsize) ([]byte, error) {
	db.DLPrintf("FSCLNT", "Read %v %v\n", fd, cnt)
	fsc.Lock()
	fdst, error := fsc.lookupStL(fd)
	//p := fsc.fids[fdst.fid]
	//version := p.lastqid().Version
	//v := fdst.mode&np.OVERSION == np.OVERSION
	if error != nil {
		fsc.Unlock()
		return nil, error
	}
	fsc.Unlock()
	reply, err := fsc.clnt(fdst.fid).Read(fdst.fid, fdst.offset, cnt)
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
	return reply.Data, nil
}

func (fsc *FsClient) Write(fd int, data []byte) (np.Tsize, error) {
	db.DLPrintf("FSCLNT", "Write %v %v\n", fd, len(data))
	fsc.Lock()
	fdst, error := fsc.lookupStL(fd)
	if error != nil {
		fsc.Unlock()
		return 0, error
	}
	fsc.Unlock()

	reply, err := fsc.clnt(fdst.fid).Write(fdst.fid, fdst.offset, data)
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

	return reply.Count, nil
}

func (fsc *FsClient) Lseek(fd int, off np.Toffset) error {
	db.DLPrintf("FSCLNT", "Lseek %v %v\n", fd, off)
	fsc.Lock()
	defer fsc.Unlock()

	fdst, error := fsc.lookupStL(fd)
	if error != nil {
		return error
	}
	fdst.offset = off
	return nil
}

func (fsc *FsClient) GetFile(path string, mode np.Tmode) ([]byte, error) {
	db.DLPrintf("FSCLNT", "GetFile %v %v\n", path, mode)
	p := np.Split(path)
	fid, rest := fsc.mnt.resolve(p)
	if fid == np.NoFid {
		db.DLPrintf("FSCLNT", "GetFile: mount -> unknown fid\n")
		if fsc.mnt.hasExited() {
			return nil, np.MkErr(np.TErrEOF, path)
		}
		return nil, np.MkErr(np.TErrNotfound, fmt.Sprintf("no mount for %v\n", path))
	}
	reply, err := fsc.clnt(fid).GetFile(fid, rest, mode, 0, 0)
	if err != nil {
		// If server could only partially resolve name, it may
		// have been because name contained a symbolic link
		// for a server; retry with resolving name.
		if np.IsDirNotFound(err) {
			fid, err = fsc.walkManyUmount(p, np.EndSlash(path), nil)
			if err != nil {
				return nil, err
			}
			reply, err = fsc.clnt(fid).GetFile(fid, []string{}, mode, 0, 0)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return reply.Data, nil
}

// Create or write file (if perm = 0).
func (fsc *FsClient) SetFile(path string, mode np.Tmode, perm np.Tperm, data []byte) (np.Tsize, error) {
	db.DLPrintf("FSCLNT", "SetFile %v %v\n", path, mode)
	p := np.Split(path)
	dir := np.Dir(p)
	base := []string{np.Base(p)}
	fid, rest := fsc.mnt.resolve(p)
	if fid == np.NoFid {
		db.DLPrintf("FSCLNT", "SetFile: mount -> unknown fid\n")
		if fsc.mnt.hasExited() {
			return 0, np.MkErr(np.TErrEOF, path)
		}
		return 0, np.MkErr(np.TErrNotfound, fmt.Sprintf("no mount for %v\n", path))
	}
	reply, err := fsc.clnt(fid).SetFile(fid, rest, mode, perm, 0, data)
	if err != nil {
		// If server could only partially resolve name, it may
		// have been because name contained a symbolic link
		// for a server; retry with resolving name.
		if np.IsDirNotFound(err) {
			if perm == 0 {
				dir = p
				base = []string{"."}
			}
			fid, err = fsc.walkManyUmount(dir, true, nil)
			if err != nil {
				return 0, err
			}
			reply, err = fsc.clnt(fid).SetFile(fid, base, mode, perm, 0, data)
			if err != nil {
				return 0, err
			}
		} else {
			return 0, err
		}
	}
	return reply.Count, nil
}

func (fsc *FsClient) ShutdownFs(name string) error {
	db.DLPrintf("FSCLNT", "ShutdownFs %v\n", name)
	path := np.Split(name)
	fid, err := fsc.walkMany(path, true, nil)
	if err != nil {
		return err
	}
	err = fsc.clnt(fid).RemoveFile(fid, []string{".exit"})
	if err != nil {
		return err
	}
	if err := fsc.Umount(path); err != nil {
		return err
	}
	return nil
}
