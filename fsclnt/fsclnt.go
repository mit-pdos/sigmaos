package fsclnt

import (
	"fmt"
	"log"
	"strings"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/protclnt"
)

//
// Sigma file system API.  A sigma proc typically has one fsclnt
// through which it interacts with all the file servers in a sigma
// deployment.  Alternatively (and not tested), a kernel could have
// one fsclnt for all its procs, but then the servers must trust the
// kernel to ensure that a proc uses only its fids; this allows many
// procs to use a single TCP connection to a server.
//

type Watch func(string, error)

type FsClient struct {
	fds   *FdTable
	fids  *FidMap
	pc    *protclnt.Clnt
	mnt   *MntTable
	uname string
}

func MakeFsClient(uname string) *FsClient {
	fsc := &FsClient{}
	fsc.fds = mkFdTable()
	fsc.fids = mkFidMap()
	fsc.mnt = makeMntTable()
	fsc.pc = protclnt.MakeClnt()
	fsc.uname = uname
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
	str += fmt.Sprintf("fids %v\n", fsc.fids)
	return str
}

func (fsc *FsClient) Uname() string {
	return fsc.uname
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
	fsc.fids.clnt(fid).Disconnect()
	return nil
}

func (fsc *FsClient) mount(fid np.Tfid, path string) *np.Err {
	p := fsc.fids.lookup(fid)
	if p == nil {
		return np.MkErr(np.TErrUnknownfid, "mount")
	}
	db.DLPrintf("FSCLNT", "Mount %v at %v %v\n", fid, path, fsc.fids.clnt(fid))
	error := fsc.mnt.add(np.Split(path), fid)
	if error != nil {
		log.Printf("%v: mount %v err %v\n", proc.GetProgram(), path, error)
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
	fid, error := fsc.fds.lookup(fd)
	if error != nil {
		return error
	}
	err := fsc.fids.clnt(fid).Clunk(fid)
	if err != nil {
		return err
	}
	fsc.fids.freeFid(fid)
	fsc.fds.closefd(fd)
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

// XXX a version that finds server based on pathname?
func (fsc *FsClient) attachReplicas(server []string, path, tree string) (np.Tfid, *np.Err) {
	p := np.Split(path)
	fid := fsc.fids.allocFid()
	ch, err := fsc.attachChannel(fid, server, p, np.Split(tree))
	if err != nil {
		return np.NoFid, err
	}
	fsc.fids.addFid(fid, ch)
	db.DLPrintf("FSCLNT", "Attach -> fid %v %v\n", fid, fsc.fids.lookup(fid))
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
	_, err = fsc.fids.clnt(fid).Open(fid, mode)
	if err != nil {
		return np.Tfence{}, err
	}
	defer fsc.clunkFid(fid)
	reply, err := fsc.fids.clnt(fid).MkFence(fid)
	if err != nil {
		log.Printf("%v: MkFence %v err %v\n", proc.GetProgram(), fid, err)
		return np.Tfence{}, err
	}
	return reply.Fence, nil
}

func (fsc *FsClient) RegisterFence(f np.Tfence, path string) error {
	p := np.Split(path)
	fid, err := fsc.walkManyUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return err
	}
	if err := fsc.fids.clnt(fid).RegisterFence(f, fid); err != nil {
		return err
	}
	return nil
}

func (fsc *FsClient) DeregisterFence(f np.Tfence, path string) error {
	p := np.Split(path)
	fid, err := fsc.walkManyUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return err
	}
	if err := fsc.fids.clnt(fid).DeregisterFence(f, fid); err != nil {
		return err
	}
	return nil
}

func (fsc *FsClient) RmFence(f np.Tfence, path string) error {
	p := np.Split(path)
	fid, err := fsc.walkManyUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return err
	}
	if err := fsc.fids.clnt(fid).RmFence(f, fid); err != nil {
		return err
	}
	return nil
}

func (fsc *FsClient) clone(fid np.Tfid) (np.Tfid, *np.Err) {
	db.DLPrintf("FSCLNT", "clone: %v %v\n", fid, fsc.fids.path(fid))
	fid2 := fsc.fids.path(fid)
	if fid2 == nil {
		return np.NoFid, np.MkErr(np.TErrNotfound, "clone")
	}
	path := fid2.copyPath()
	fid1 := fsc.fids.allocFid()
	_, err := fid2.pc.Walk(fid, fid1, nil)
	if err != nil {
		// XXX free fid
		return np.NoFid, err
	}
	db.DLPrintf("FSCLNT", "clone: %v %v -> %v\n", fid, fsc.fids.lookup(fid), fid1)
	fsc.fids.addFid(fid1, path)
	return fid1, nil
}

func (fsc *FsClient) clunkFid(fid np.Tfid) {
	err := fsc.fids.clnt(fid).Clunk(fid)
	if err != nil {
		db.DLPrintf("FSCLNT", "clunkFid clunk failed %v\n", err)
	}
	fsc.fids.freeFid(fid)
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
	reply, err := fsc.fids.clnt(fid).Create(fid, base, perm, mode)
	if err != nil {
		return -1, err
	}
	fsc.fids.path(fid).add(base, reply.Qid)
	fd := fsc.fds.allocFd(fid, mode)
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
	_, err = fsc.fids.clnt(fid).Wstat(fid, st)
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
	if fsc.fids.clnt(fid) != fsc.fids.clnt(fid1) {
		return np.MkErr(np.TErrInval, "paths at different servers")
	}
	if _, err = fsc.fids.clnt(fid).Renameat(fid, o, fid1, n); err != nil {
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
	fsc.fids.freeFid(fid2)
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
	err := fsc.fids.clnt(fid).RemoveFile(fid, rest)
	if err != nil {
		// If server could only partially resolve name, it may
		// have been because name contained a symbolic link
		// for a server; retry with resolving name.
		if np.IsDirNotFound(err) {
			fid, err = fsc.walkManyUmount(path, np.EndSlash(name), nil)
			if err != nil {
				return err
			}
			err = fsc.fids.clnt(fid).Remove(fid)
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
		st.Name = strings.Join(fsc.fids.clnt(target).Server(), ",")
		return st, nil
	} else {
		fid, err := fsc.walkMany(np.Split(name), np.EndSlash(name), nil)
		if err != nil {
			return nil, err
		}
		reply, err := fsc.fids.clnt(fid).Stat(fid)
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
	_, err = fsc.fids.clnt(fid).Open(fid, mode)
	if err != nil {
		return -1, err
	}
	// XXX check reply.Qid?
	fd := fsc.fds.allocFd(fid, mode)
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
		version := fsc.fids.path(fid).lastqid().Version
		err := fsc.fids.clnt(fid).Watch(fid, nil, version)
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
		version := fsc.fids.path(fid).lastqid().Version
		if err := fsc.fids.clnt(fid).Watch(fid, nil, version); err != nil {
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
	fid, off, error := fsc.fds.lookupOff(fd)
	//p := fsc.fids[fdst.fid]
	//version := p.lastqid().Version
	//v := fdst.mode&np.OVERSION == np.OVERSION
	if error != nil {
		return nil, error
	}
	reply, err := fsc.fids.clnt(fid).Read(fid, off, cnt)
	if err != nil {
		return nil, err
	}
	fsc.fds.incOff(fd, np.Toffset(len(reply.Data)))
	db.DLPrintf("FSCLNT", "Read %v -> %v %v\n", fd, reply, err)
	return reply.Data, nil
}

func (fsc *FsClient) Write(fd int, data []byte) (np.Tsize, error) {
	db.DLPrintf("FSCLNT", "Write %v %v\n", fd, len(data))
	fid, off, error := fsc.fds.lookupOff(fd)
	if error != nil {
		return 0, error
	}
	reply, err := fsc.fids.clnt(fid).Write(fid, off, data)
	if err != nil {
		return 0, err
	}
	fsc.fds.incOff(fd, np.Toffset(reply.Count))
	return reply.Count, nil
}

func (fsc *FsClient) Lseek(fd int, off np.Toffset) error {
	db.DLPrintf("FSCLNT", "Lseek %v %v\n", fd, off)
	err := fsc.fds.setOffset(fd, off)
	if err != nil {
		return err
	}
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
	reply, err := fsc.fids.clnt(fid).GetFile(fid, rest, mode, 0, 0)
	if err != nil {
		// If server could only partially resolve name, it may
		// have been because name contained a symbolic link
		// for a server; retry with resolving name.
		if np.IsDirNotFound(err) {
			fid, err = fsc.walkManyUmount(p, np.EndSlash(path), nil)
			if err != nil {
				return nil, err
			}
			reply, err = fsc.fids.clnt(fid).GetFile(fid, []string{}, mode, 0, 0)
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
	reply, err := fsc.fids.clnt(fid).SetFile(fid, rest, mode, perm, 0, data)
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
			reply, err = fsc.fids.clnt(fid).SetFile(fid, base, mode, perm, 0, data)
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
	err = fsc.fids.clnt(fid).RemoveFile(fid, []string{".exit"})
	if err != nil {
		return err
	}
	if err := fsc.Umount(path); err != nil {
		return err
	}
	return nil
}
