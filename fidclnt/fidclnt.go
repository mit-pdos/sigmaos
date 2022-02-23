package fidclnt

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
// Sigma file system API at the level of fids. A proc has one
// FidClient through which it interacts with all the file servers in a
// sigma deployment.  The FidClient API is wrapped by FdClient using
// file descriptors to allow sharing of a FidClient among several
// procs.

type Watch func(string, error)

type FidClient struct {
	fids *FidMap
	pc   *protclnt.Clnt
	mnt  *MntTable
}

func MakeFidClient() *FidClient {
	fsc := &FidClient{}
	fsc.fids = mkFidMap()
	fsc.mnt = makeMntTable()
	fsc.pc = protclnt.MakeClnt()
	return fsc
}

func (fsc *FidClient) ReadSeqNo() np.Tseqno {
	return fsc.pc.ReadSeqNo()
}

func (fsc *FidClient) Exit() {
	fsc.mnt.exit()
	fsc.pc.Exit()
}

func (fsc *FidClient) String() string {
	str := fmt.Sprintf("Fsclnt fid table:\n")
	str += fmt.Sprintf("fids %v\n", fsc.fids)
	return str
}

// Simulate network partition to server that exports path
func (fsc *FidClient) Disconnect(path string) error {
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

func (fsc *FidClient) mount(fid np.Tfid, path string) *np.Err {
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

func (fsc *FidClient) Mount(fid np.Tfid, path string) error {
	if err := fsc.mount(fid, path); err != nil {
		return err
	}
	return nil
}

func (fsc *FidClient) Close(fid np.Tfid) error {
	err := fsc.fids.clnt(fid).Clunk(fid)
	if err != nil {
		return err
	}
	fsc.fids.freeFid(fid)
	return nil
}

func (fsc *FidClient) attachChannel(fid np.Tfid, uname string, server []string, p []string, tree []string) (*Path, *np.Err) {
	reply, err := fsc.pc.Attach(server, uname, fid, tree)
	if err != nil {
		return nil, err
	}
	ch := fsc.pc.MakeProtClnt(server)
	return makePath(ch, uname, p, []np.Tqid{reply.Qid}), nil
}

// XXX a version that finds server based on pathname?
func (fsc *FidClient) attachReplicas(uname string, server []string, path, tree string) (np.Tfid, *np.Err) {
	p := np.Split(path)
	fid := fsc.fids.allocFid()
	ch, err := fsc.attachChannel(fid, uname, server, p, np.Split(tree))
	if err != nil {
		return np.NoFid, err
	}
	fsc.fids.addFid(fid, ch)
	db.DLPrintf("FSCLNT", "Attach -> fid %v %v\n", fid, fsc.fids.lookup(fid))
	return fid, nil
}

func (fsc *FidClient) attach(uname string, server, path, tree string) (np.Tfid, *np.Err) {
	return fsc.attachReplicas(uname, []string{server}, path, tree)
}

func (fsc *FidClient) AttachReplicas(uname string, server []string, path, tree string) (np.Tfid, error) {
	if fid, err := fsc.attachReplicas(uname, server, path, tree); err != nil {
		return np.NoFid, err
	} else {
		return fid, nil
	}
}

func (fsc *FidClient) clone(fid np.Tfid) (np.Tfid, *np.Err) {
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

func (fsc *FidClient) clunkFid(fid np.Tfid) {
	err := fsc.fids.clnt(fid).Clunk(fid)
	if err != nil {
		db.DLPrintf("FSCLNT", "clunkFid clunk failed %v\n", err)
	}
	fsc.fids.freeFid(fid)
}

func (fsc *FidClient) Create(path string, perm np.Tperm, mode np.Tmode) (np.Tfid, error) {
	db.DLPrintf("FSCLNT", "Create %v perm %v\n", path, perm)
	p := np.Split(path)
	dir := np.Dir(p)
	base := np.Base(p)
	fid, err := fsc.walkMany(dir, true, nil)
	if err != nil {
		return np.NoFid, err
	}
	reply, err := fsc.fids.clnt(fid).Create(fid, base, perm, mode)
	if err != nil {
		return np.NoFid, err
	}
	fsc.fids.path(fid).add(base, reply.Qid)
	return fid, nil
}

// Rename using renameat() for across directories or using wstat()
// for within a directory.
func (fsc *FidClient) Rename(old string, new string) error {
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
func (fsc *FidClient) renameat(old, new string) *np.Err {
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

func (fsc *FidClient) Umount(path []string) error {
	db.DLPrintf("FSCLNT", "Umount %v\n", path)
	fid2, err := fsc.mnt.umount(path)
	if err != nil {
		return err
	}
	fsc.fids.freeFid(fid2)
	return nil
}

func (fsc *FidClient) Remove(name string) error {
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
	err := fsc.fids.clnt(fid).RemoveFile(fid, rest, np.EndSlash(name))
	if err != nil {
		if np.IsMaybeSpecialElem(err) {
			fid, err = fsc.walkManyUmount(path, np.EndSlash(name), nil)
			if err != nil {
				return err
			}
			defer fsc.clunkFid(fid)
			err = fsc.fids.clnt(fid).Remove(fid)
		}
	}
	if err != nil {
		return err
	}
	return nil
}

func (fsc *FidClient) Stat(name string) (*np.Stat, error) {
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
		defer fsc.clunkFid(fid)
		reply, err := fsc.fids.clnt(fid).Stat(fid)
		if err != nil {
			return nil, err
		}
		return &reply.Stat, nil
	}
}

// XXX clone fid?
func (fsc *FidClient) readlink(pc *protclnt.ProtClnt, fid np.Tfid) (string, *np.Err) {
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

func (fsc *FidClient) OpenWatch(path string, mode np.Tmode, w Watch) (np.Tfid, error) {
	db.DLPrintf("FSCLNT", "Open %v %v\n", path, mode)
	p := np.Split(path)
	fid, err := fsc.walkManyUmount(p, np.EndSlash(path), w)
	if err != nil {
		return np.NoFid, err
	}
	_, err = fsc.fids.clnt(fid).Open(fid, mode)
	if err != nil {
		return np.NoFid, err
	}
	// XXX check reply.Qid?
	return fid, nil
}

func (fsc *FidClient) Open(path string, mode np.Tmode) (np.Tfid, error) {
	return fsc.OpenWatch(path, mode, nil)
}

func (fsc *FidClient) SetDirWatch(path string, w Watch) error {
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

func (fsc *FidClient) SetRemoveWatch(path string, w Watch) error {
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

func (fsc *FidClient) Read(fid np.Tfid, off np.Toffset, cnt np.Tsize) ([]byte, error) {
	db.DLPrintf("FSCLNT", "Read %v %v\n", fid, cnt)
	//p := fsc.fids[fdst.fid]
	//version := p.lastqid().Version
	//v := fdst.mode&np.OVERSION == np.OVERSION
	reply, err := fsc.fids.clnt(fid).Read(fid, off, cnt)
	if err != nil {
		return nil, err
	}
	db.DLPrintf("FSCLNT", "Read %v -> %v %v\n", fid, reply, err)
	return reply.Data, nil
}

func (fsc *FidClient) Write(fid np.Tfid, off np.Toffset, data []byte) (np.Tsize, error) {
	db.DLPrintf("FSCLNT", "Write %v %v\n", fid, len(data))
	reply, err := fsc.fids.clnt(fid).Write(fid, off, data)
	if err != nil {
		return 0, err
	}
	return reply.Count, nil
}

func (fsc *FidClient) GetFile(path string, mode np.Tmode, off np.Toffset, cnt np.Tsize) ([]byte, error) {
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
	// Optimistcally GetFile without doing a pathname
	// walk; this may fail if rest contains an automount
	// symlink.
	reply, err := fsc.fids.clnt(fid).GetFile(fid, rest, mode, off, cnt, np.EndSlash(path))
	if err != nil {
		if np.IsMaybeSpecialElem(err) {
			fid, err = fsc.walkManyUmount(p, np.EndSlash(path), nil)
			if err != nil {
				return nil, err
			}
			defer fsc.clunkFid(fid)
			reply, err = fsc.fids.clnt(fid).GetFile(fid, []string{}, mode, off, cnt, false)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return reply.Data, nil
}

// Write file
func (fsc *FidClient) SetFile(path string, mode np.Tmode, data []byte, off np.Toffset) (np.Tsize, error) {
	db.DLPrintf("FSCLNT", "SetFile %v %v\n", path, mode)
	p := np.Split(path)
	fid, rest := fsc.mnt.resolve(p)
	if fid == np.NoFid {
		db.DLPrintf("FSCLNT", "SetFile: mount -> unknown fid\n")
		if fsc.mnt.hasExited() {
			return 0, np.MkErr(np.TErrEOF, path)
		}
		return 0, np.MkErr(np.TErrNotfound, fmt.Sprintf("no mount for %v\n", path))
	}
	// Optimistcally SetFile without doing a pathname walk; this
	// may fail if rest contains an automount symlink.
	// XXX On EOF try another replica for TestMaintainReplicationLevelCrashProcd
	reply, err := fsc.fids.clnt(fid).SetFile(fid, rest, mode, off, data, np.EndSlash(path))
	if err != nil {
		if np.IsMaybeSpecialElem(err) || np.IsErrEOF(err) {
			fid, err = fsc.walkManyUmount(p, np.EndSlash(path), nil)
			if err != nil {
				return 0, err
			}
			defer fsc.clunkFid(fid)
			reply, err = fsc.fids.clnt(fid).SetFile(fid, []string{}, mode, off, data, false)
			if err != nil {
				return 0, err
			}
		} else {
			return 0, err
		}
	}
	return reply.Count, nil
}

// Create file
func (fsc *FidClient) PutFile(path string, mode np.Tmode, perm np.Tperm, data []byte, off np.Toffset) (np.Tsize, error) {
	db.DLPrintf("FSCLNT", "PutFile %v %v\n", path, mode)
	p := np.Split(path)
	fid, rest := fsc.mnt.resolve(p)
	if fid == np.NoFid {
		db.DLPrintf("FSCLNT", "PutFile: mount -> unknown fid\n")
		if fsc.mnt.hasExited() {
			return 0, np.MkErr(np.TErrEOF, path)
		}
		return 0, np.MkErr(np.TErrNotfound, fmt.Sprintf("no mount for %v\n", path))
	}
	// Optimistcally PutFile without doing a pathname
	// walk; this may fail if rest contains an automount
	// symlink.
	reply, err := fsc.fids.clnt(fid).PutFile(fid, rest, mode, perm, off, data)
	if err != nil {
		if np.IsMaybeSpecialElem(err) || np.IsErrEOF(err) {
			dir := np.Dir(p)
			base := []string{np.Base(p)}
			fid, err = fsc.walkManyUmount(dir, true, nil)
			if err != nil {
				return 0, err
			}
			defer fsc.clunkFid(fid)
			reply, err = fsc.fids.clnt(fid).PutFile(fid, base, mode, perm, off, data)
			if err != nil {
				return 0, err
			}
		} else {
			return 0, err
		}
	}
	return reply.Count, nil
}

func (fsc *FidClient) MakeFence(path string, mode np.Tmode) (np.Tfence, error) {
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

func (fsc *FidClient) RegisterFence(f np.Tfence, path string) error {
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

func (fsc *FidClient) DeregisterFence(f np.Tfence, path string) error {
	p := np.Split(path)
	fid, err := fsc.walkManyUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return err
	}
	defer fsc.clunkFid(fid)
	if err := fsc.fids.clnt(fid).DeregisterFence(f, fid); err != nil {
		return err
	}
	return nil
}

func (fsc *FidClient) RmFence(f np.Tfence, path string) error {
	p := np.Split(path)
	fid, err := fsc.walkManyUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return err
	}
	defer fsc.clunkFid(fid)
	if err := fsc.fids.clnt(fid).RmFence(f, fid); err != nil {
		return err
	}
	return nil
}
