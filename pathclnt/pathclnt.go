package pathclnt

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/reader"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/writer"
)

//
// The Sigma file system API at the level of pathnames.  The
// pathname-based operations are implemented here and support dynamic
// mounts, sfs-like pathnames, etc. All fid-based operations are
// inherited from FidClnt.
//

type Watch func(string, error)

type PathClnt struct {
	*fidclnt.FidClnt
	mnt     *MntTable
	rootmt  *RootMountTable
	chunkSz sessp.Tsize
	realm   sp.Trealm
	lip     string
	cid     sp.TclntId
}

func MakePathClnt(fidc *fidclnt.FidClnt, clntnet string, realm sp.Trealm, lip string, sz sessp.Tsize) *PathClnt {
	pathc := &PathClnt{mnt: makeMntTable(), chunkSz: sz, realm: realm, lip: lip}
	if fidc == nil {
		pathc.FidClnt = fidclnt.MakeFidClnt(clntnet)
	} else {
		pathc.FidClnt = fidc
	}
	pathc.rootmt = mkRootMountTable()
	pathc.cid = sp.TclntId(rand.Uint64())
	return pathc
}

func (pathc *PathClnt) String() string {
	str := fmt.Sprintf("Pathclnt mount table:\n")
	str += fmt.Sprintf("%v\n", pathc.mnt)
	return str
}

func (pathc *PathClnt) Realm() sp.Trealm {
	return pathc.realm
}

func (pathc *PathClnt) ClntID() sp.TclntId {
	return pathc.cid
}

func (pathc *PathClnt) GetLocalIP() string {
	return pathc.lip
}

func (pathc *PathClnt) SetChunkSz(sz sessp.Tsize) {
	pathc.chunkSz = sz
}

func (pathc *PathClnt) GetChunkSz() sessp.Tsize {
	return pathc.chunkSz
}

func (pathc *PathClnt) Mounts() []string {
	return pathc.mnt.mountedPaths()
}

func (pathc *PathClnt) MountTree(uname sp.Tuname, addrs sp.Taddrs, tree, mnt string) error {
	if fd, err := pathc.Attach(uname, pathc.cid, addrs, "", tree); err == nil {
		return pathc.Mount(fd, mnt)
	} else {
		return err
	}
}

// Return path to the symlink for the last server on this path and the
// the rest of the path on the server.
func (pathc *PathClnt) PathLastSymlink(pn string, uname sp.Tuname) (path.Path, path.Path, error) {
	// Automount the longest prefix of pn; if pn exist, then the
	// server holding the directory/file correspending to pn.
	if _, err := pathc.Stat(pn+"/", uname); err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "Stat %v err %v\n", pn, err)
	}
	return pathc.LastMount(pn, uname)
}

// Close all sessions
func (pathc *PathClnt) DetachAll() error {
	db.DPrintf(db.PATHCLNT, "%v: Fslib.DetachAll\n", proc.GetPid())
	return pathc.FidClnt.DetachAll(pathc.cid)
}

// Detach from server. XXX Mixes up umount a file system at server and
// closing session; if two mounts point to the same server; the first
// detach will close the session regardless of the second mount point.
func (pathc *PathClnt) Detach(pn string) error {
	fid, _, err := pathc.mnt.umount(path.Split(pn), true)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Free(fid)
	if err := pathc.FidClnt.Detach(fid, pathc.cid); err != nil {
		return err
	}
	return nil
}

// Simulate network partition to server that exports path
func (pathc *PathClnt) Disconnect(pn string) error {
	fid, _, err := pathc.mnt.umount(path.Split(pn), true)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Free(fid)
	if err := pathc.FidClnt.Lookup(fid).Disconnect(); err != nil {
		return err
	}
	return nil
}

func (pathc *PathClnt) MakeReader(fid sp.Tfid, path string, chunksz sessp.Tsize) *reader.Reader {
	return reader.MakeReader(pathc.FidClnt, path, fid, chunksz)
}

func (pathc *PathClnt) MakeWriter(fid sp.Tfid) *writer.Writer {
	return writer.MakeWriter(pathc.FidClnt, fid)
}

func (pathc *PathClnt) readlink(fid sp.Tfid) ([]byte, *serr.Err) {
	qid := pathc.Qid(fid)
	if qid.Ttype()&sp.QTSYMLINK == 0 {
		return nil, serr.MkErr(serr.TErrNotSymlink, qid.Type)
	}
	_, err := pathc.FidClnt.Open(fid, sp.OREAD)
	if err != nil {
		return nil, err
	}
	rdr := reader.MakeReader(pathc.FidClnt, "", fid, pathc.chunkSz)
	b, err := rdr.GetDataErr()
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (pathc *PathClnt) mount(fid sp.Tfid, pn string) *serr.Err {
	if err := pathc.mnt.add(path.Split(pn), fid); err != nil {
		if err.Code() == serr.TErrExists {
			// Another thread may already have mounted
			// path; clunk the fid and don't return an
			// error.
			pathc.Clunk(fid)
			return nil
		} else {
			return err
		}
	}
	return nil
}

func (pathc *PathClnt) Mount(fid sp.Tfid, path string) error {
	if err := pathc.mount(fid, path); err != nil {
		return err
	}
	return nil
}

func (pathc *PathClnt) Create(p string, uname sp.Tuname, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId) (sp.Tfid, error) {
	db.DPrintf(db.PATHCLNT, "Create %v perm %v lid %v\n", p, perm, lid)
	path := path.Split(p)
	dir := path.Dir()
	base := path.Base()
	fid, err := pathc.walk(dir, uname, true, nil)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "Walk failed: %v", p)
		return sp.NoFid, err
	}
	fid, err = pathc.FidClnt.Create(fid, base, perm, mode, lid)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "create failed: %v", p)
		return sp.NoFid, err
	}
	return fid, nil
}

// Rename using renameat() for across directories or using wstat()
// for within a directory.
func (pathc *PathClnt) Rename(old, new string, uname sp.Tuname) error {
	db.DPrintf(db.PATHCLNT, "Rename %v %v\n", old, new)
	opath := path.Split(old)
	npath := path.Split(new)

	if len(opath) != len(npath) {
		if err := pathc.renameat(old, new, uname); err != nil {
			return err
		}
		return nil
	}
	for i, n := range opath[:len(opath)-1] {
		if npath[i] != n {
			if err := pathc.renameat(old, new, uname); err != nil {
				return err
			}
			return nil
		}
	}
	fid, err := pathc.walk(opath, uname, path.EndSlash(old), nil)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Clunk(fid)
	st := sp.MkStatNull()
	st.Name = npath[len(npath)-1]
	err = pathc.FidClnt.Wstat(fid, st)
	if err != nil {
		return err
	}
	return nil
}

// Rename across directories of a single server using Renameat
func (pathc *PathClnt) renameat(old, new string, uname sp.Tuname) *serr.Err {
	db.DPrintf(db.PATHCLNT, "Renameat %v %v\n", old, new)
	opath := path.Split(old)
	npath := path.Split(new)
	o := opath[len(opath)-1]
	n := npath[len(npath)-1]
	fid, err := pathc.walk(opath[:len(opath)-1], uname, path.EndSlash(old), nil)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Clunk(fid)
	fid1, err := pathc.walk(npath[:len(npath)-1], uname, path.EndSlash(old), nil)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Clunk(fid1)
	return pathc.FidClnt.Renameat(fid, o, fid1, n)
}

func (pathc *PathClnt) Remove(name string, uname sp.Tuname) error {
	db.DPrintf(db.PATHCLNT, "Remove %v\n", name)
	pn := path.Split(name)
	fid, rest, err := pathc.resolve(pn, uname, path.EndSlash(name))
	if err != nil {
		return err
	}

	// Optimistcally remove obj without doing a pathname
	// walk; this may fail if rest contains an automount
	// symlink.
	err = pathc.FidClnt.RemoveFile(fid, rest, path.EndSlash(name))
	if err != nil {
		if err.IsMaybeSpecialElem() || err.IsErrUnreachable() {
			fid, err = pathc.walk(pn, uname, path.EndSlash(name), nil)
			if err != nil {
				return err
			}
			defer pathc.FidClnt.Clunk(fid)
			err = pathc.FidClnt.Remove(fid)
		}
	}
	if err != nil {
		return err
	}
	return nil
}

func (pathc *PathClnt) Stat(name string, uname sp.Tuname) (*sp.Stat, error) {
	db.DPrintf(db.PATHCLNT, "Stat %v\n", name)
	pn := path.Split(name)
	// XXX ignore err?
	target, rest, _ := pathc.resolve(pn, uname, true)
	if len(rest) == 0 && !path.EndSlash(name) {
		st := sp.MkStatNull()
		st.Name = pathc.FidClnt.Lookup(target).Servers().String()
		return st, nil
	} else {
		fid, err := pathc.walk(path.Split(name), uname, path.EndSlash(name), nil)
		if err != nil {
			return nil, err
		}
		defer pathc.FidClnt.Clunk(fid)
		st, err := pathc.FidClnt.Stat(fid)
		if err != nil {
			return nil, err
		}
		return st, nil
	}
}

func (pathc *PathClnt) OpenWatch(pn string, uname sp.Tuname, mode sp.Tmode, w Watch) (sp.Tfid, error) {
	db.DPrintf(db.PATHCLNT, "Open %v %v\n", pn, mode)
	p := path.Split(pn)
	fid, err := pathc.walk(p, uname, path.EndSlash(pn), w)
	if err != nil {
		return sp.NoFid, err
	}
	_, err = pathc.FidClnt.Open(fid, mode)
	if err != nil {
		return sp.NoFid, err
	}
	return fid, nil
}

func (pathc *PathClnt) Open(path string, uname sp.Tuname, mode sp.Tmode) (sp.Tfid, error) {
	return pathc.OpenWatch(path, uname, mode, nil)
}

func (pathc *PathClnt) SetDirWatch(fid sp.Tfid, path string, w Watch) error {
	db.DPrintf(db.PATHCLNT, "SetDirWatch %v\n", fid)
	go func() {
		err := pathc.FidClnt.Watch(fid)
		db.DPrintf(db.PATHCLNT, "SetDirWatch: Watch returns %v %v\n", fid, err)
		if err == nil {
			w(path, nil)
		} else {
			w(path, err)
		}
	}()
	return nil
}

func (pathc *PathClnt) SetRemoveWatch(pn string, uname sp.Tuname, w Watch) error {
	db.DPrintf(db.PATHCLNT, "SetRemoveWatch %v", pn)
	p := path.Split(pn)
	fid, err := pathc.walk(p, uname, path.EndSlash(pn), nil)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "SetRemoveWatch: Walk %v err %v", pn, err)
		return err
	}
	if w == nil {
		return serr.MkErr(serr.TErrInval, "watch")
	}
	go func() {
		err := pathc.FidClnt.Watch(fid)
		db.DPrintf(db.PATHCLNT, "SetRemoveWatch: Watch %v %v err %v\n", fid, pn, err)
		if err == nil {
			w(pn, nil)
		} else {
			w(pn, err)
		}
		pathc.Clunk(fid)
	}()
	return nil
}

func (pathc *PathClnt) GetFile(pn string, uname sp.Tuname, mode sp.Tmode, off sp.Toffset, cnt sessp.Tsize) ([]byte, error) {
	db.DPrintf(db.PATHCLNT, "GetFile %v %v\n", pn, mode)
	p := path.Split(pn)
	fid, rest, err := pathc.resolve(p, uname, path.EndSlash(pn))
	if err != nil {
		return nil, err
	}
	// Optimistcally GetFile without doing a pathname
	// walk; this may fail if rest contains an automount
	// symlink or if server is unreachable.
	data, err := pathc.FidClnt.GetFile(fid, rest, mode, off, cnt, path.EndSlash(pn))
	if err != nil {
		if err.IsMaybeSpecialElem() || err.IsErrUnreachable() {
			fid, err = pathc.walk(p, uname, path.EndSlash(pn), nil)
			if err != nil {
				return nil, err
			}
			defer pathc.FidClnt.Clunk(fid)
			data, err = pathc.FidClnt.GetFile(fid, []string{}, mode, off, cnt, false)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return data, nil
}

// Create or open file and write it
func (pathc *PathClnt) PutFile(pn string, uname sp.Tuname, mode sp.Tmode, perm sp.Tperm, data []byte, off sp.Toffset, lid sp.TleaseId) (sessp.Tsize, error) {
	db.DPrintf(db.PATHCLNT, "PutFile %v %v %v\n", pn, mode, lid)
	p := path.Split(pn)
	fid, rest, err := pathc.resolve(p, uname, path.EndSlash(pn))
	if err != nil {
		return 0, err
	}
	// Optimistcally PutFile without doing a pathname
	// walk; this may fail if rest contains an automount
	// symlink or if server is unreachable.
	cnt, err := pathc.FidClnt.PutFile(fid, rest, mode, perm, off, data, path.EndSlash(pn), lid)
	if err != nil {
		if err.IsMaybeSpecialElem() || err.IsErrUnreachable() {
			dir := p.Dir()
			base := path.Path{p.Base()}
			resolve := true
			if p.Base() == err.Obj { // was the final pn component a symlink?
				dir = p
				base = path.Path{}
				resolve = path.EndSlash(pn)
			}
			fid, err = pathc.walk(dir, uname, resolve, nil)
			if err != nil {
				return 0, err
			}
			defer pathc.FidClnt.Clunk(fid)
			cnt, err = pathc.FidClnt.PutFile(fid, base, mode, perm, off, data, false, lid)
			if err != nil {
				return 0, err
			}
		} else {
			return 0, err
		}
	}
	return cnt, nil
}

func (pathc *PathClnt) resolve(p path.Path, uname sp.Tuname, resolve bool) (sp.Tfid, path.Path, *serr.Err) {
	pathc.resolveRoot(p, uname)
	return pathc.mnt.resolve(p, resolve)
}

func (pathc *PathClnt) LastMount(pn string, uname sp.Tuname) (path.Path, path.Path, error) {
	p := path.Split(pn)
	_, left, err := pathc.resolve(p, uname, path.EndSlash(pn))
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "resolve  %v err %v\n", pn, err)
		return nil, nil, err
	}
	p = p[0 : len(p)-len(left)]
	return p, left, nil
}
