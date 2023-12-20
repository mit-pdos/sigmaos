package pathclnt

import (
	"errors"
	"fmt"

	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/reader"
	"sigmaos/serr"
	sos "sigmaos/sigmaos"
	sp "sigmaos/sigmap"
)

//
// The Sigma file system API at the level of pathnames.  The
// pathname-based operations are implemented here and support dynamic
// mounts, sfs-like pathnames, etc. All fid-based operations are
// inherited from FidClnt.
//

type PathClnt struct {
	*fidclnt.FidClnt
	pcfg       *proc.ProcEnv
	ndMntCache *NamedMountCache
	mnt        *MntTable
	rootmt     *RootMountTable
	realm      sp.Trealm
	lip        string
	cid        sp.TclntId
}

func NewPathClnt(pcfg *proc.ProcEnv, fidc *fidclnt.FidClnt) *PathClnt {
	pathc := &PathClnt{
		pcfg: pcfg,
		mnt:  newMntTable(),
	}
	if fidc == nil {
		pathc.FidClnt = fidclnt.NewFidClnt(pcfg.Net)
	} else {
		pathc.FidClnt = fidc
	}
	pathc.ndMntCache = NewNamedMountCache(pcfg)
	pathc.rootmt = newRootMountTable()
	pathc.cid = sp.TclntId(rand.Uint64())
	return pathc
}

func (pathc *PathClnt) String() string {
	str := fmt.Sprintf("Pathclnt mount table:\n")
	str += fmt.Sprintf("%v\n", pathc.mnt)
	return str
}

func (pathc *PathClnt) ClntId() sp.TclntId {
	return pathc.cid
}

func (pathc *PathClnt) Mounts() []string {
	return pathc.mnt.mountedPaths()
}

func (pathc *PathClnt) MountTree(uname sp.Tuname, addrs sp.Taddrs, tree, mnt string) error {
	db.DPrintf(db.PATHCLNT, "MountTree [%v]/%v mnt %v", addrs, tree, mnt)
	if fd, err := pathc.Attach(uname, pathc.cid, addrs, "", tree); err == nil {
		return pathc.Mount(fd, mnt)
	} else {
		db.DPrintf(db.PATHCLNT_ERR, "MountTree Attach [%v]/%v err %v", addrs, tree, err)
		return err
	}
}

// Return path including the last mount file on this path and the rest
// of the path on the server.
func (pathc *PathClnt) PathLastMount(pn string, uname sp.Tuname) (path.Path, path.Path, error) {
	// Automount the longest prefix of pn; if pn exist, then the
	// server holding the directory/file correspending to pn.
	if _, err := pathc.Stat(pn+"/", uname); err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "Stat %v err %v\n", pn, err)
	}
	return pathc.LastMount(pn, uname)
}

// Close all sessions
func (pathc *PathClnt) DetachAll() error {
	db.DPrintf(db.PATHCLNT, "Fslib.DetachAll\n")
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

type rdr struct {
	*fidclnt.FidClnt
	fid sp.Tfid
}

func newRdr(fdc *fidclnt.FidClnt, fid sp.Tfid) *rdr {
	return &rdr{fdc, fid}
}

func (rd *rdr) Close() error {
	return rd.FidClnt.Clunk(rd.fid)
}

func (rd *rdr) Read(o sp.Toffset, sz sp.Tsize) ([]byte, error) {
	b, err := rd.ReadF(rd.fid, o, sz)
	if err != nil {
		return b, err
	}
	return b, nil
}

func (pathc *PathClnt) NewReader(fid sp.Tfid, path string) *reader.Reader {
	return reader.NewReader(newRdr(pathc.FidClnt, fid), path)
}

func (pathc *PathClnt) readlink(fid sp.Tfid) ([]byte, *serr.Err) {
	db.DPrintf(db.PATHCLNT, "readlink %v", fid)
	qid := pathc.Qid(fid)
	if qid.Ttype()&sp.QTSYMLINK == 0 {
		return nil, serr.NewErr(serr.TErrNotSymlink, qid.Type)
	}
	_, err := pathc.FidClnt.Open(fid, sp.OREAD)
	if err != nil {
		return nil, err
	}
	rdr := reader.NewReader(newRdr(pathc.FidClnt, fid), "")
	b, r := rdr.GetDataErr()
	if errors.As(r, &err) {
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

func (pathc *PathClnt) Create(p string, uname sp.Tuname, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f sp.Tfence) (sp.Tfid, error) {
	db.DPrintf(db.PATHCLNT, "Create %v perm %v lid %v\n", p, perm, lid)
	path := path.Split(p)
	dir := path.Dir()
	base := path.Base()
	fid, err := pathc.walk(dir, uname, true, nil)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "Walk failed: %v err %v", p, err)
		return sp.NoFid, err
	}
	fid, err = pathc.FidClnt.Create(fid, base, perm, mode, lid, f)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "create failed: %v err %v", p, err)
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
	st := sp.NewStatNull()
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
	err = pathc.FidClnt.RemoveFile(fid, rest, path.EndSlash(name))
	if Retry(err) {
		fid, err = pathc.walk(pn, uname, path.EndSlash(name), nil)
		if err != nil {
			return err
		}
		defer pathc.FidClnt.Clunk(fid)
		err = pathc.FidClnt.Remove(fid)
	} else if err != nil {
		return err
	}
	return nil
}

func (pathc *PathClnt) Stat(name string, uname sp.Tuname) (*sp.Stat, error) {
	db.DPrintf(db.PATHCLNT, "Stat %v\n", name)
	pn := path.Split(name)
	target, rest, err := pathc.resolve(pn, uname, true)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Stat resolve %v err %v\n", pn, err)
	}
	db.DPrintf(db.PATHCLNT, "Stat resolve %v target %v rest %v\n", pn, target, rest)
	if len(rest) == 0 && !path.EndSlash(name) {
		st := sp.NewStatNull()
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

func (pathc *PathClnt) Open(pn string, uname sp.Tuname, mode sp.Tmode, w sos.Watch) (sp.Tfid, error) {
	db.DPrintf(db.PATHCLNT, "Open %v %v %v\n", pn, mode, w)
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

func (pathc *PathClnt) SetDirWatch(fid sp.Tfid, path string, w sos.Watch) error {
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

func (pathc *PathClnt) SetRemoveWatch(pn string, uname sp.Tuname, w sos.Watch) error {
	db.DPrintf(db.PATHCLNT, "SetRemoveWatch %v", pn)
	p := path.Split(pn)
	fid, err := pathc.walk(p, uname, path.EndSlash(pn), nil)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "SetRemoveWatch: Walk %v err %v", pn, err)
		return err
	}
	if w == nil {
		return serr.NewErr(serr.TErrInval, "watch")
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

// Several calls optimistically connect to a recently-mounted server
// without doing a pathname walk; this may fail, and the call should
// walk. retry() says when to retry.
func Retry(err *serr.Err) bool {
	if err == nil {
		return false
	}
	return err.IsErrUnreachable() || err.IsErrUnknownfid() || err.IsMaybeSpecialElem()
}

func (pathc *PathClnt) GetFile(pn string, uname sp.Tuname, mode sp.Tmode, off sp.Toffset, cnt sp.Tsize) ([]byte, error) {
	db.DPrintf(db.PATHCLNT, "GetFile %v %v\n", pn, mode)
	p := path.Split(pn)
	fid, rest, err := pathc.resolve(p, uname, path.EndSlash(pn))
	if err != nil {
		return nil, err
	}
	data, err := pathc.FidClnt.GetFile(fid, rest, mode, off, cnt, path.EndSlash(pn))
	if Retry(err) {
		fid, err = pathc.walk(p, uname, path.EndSlash(pn), nil)
		if err != nil {
			return nil, err
		}
		defer pathc.FidClnt.Clunk(fid)
		data, err = pathc.FidClnt.GetFile(fid, []string{}, mode, off, cnt, false)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return data, nil
}

// Create or open file and write it
func (pathc *PathClnt) PutFile(pn string, uname sp.Tuname, mode sp.Tmode, perm sp.Tperm, data []byte, off sp.Toffset, lid sp.TleaseId) (sp.Tsize, error) {
	db.DPrintf(db.PATHCLNT, "PutFile %v %v %v\n", pn, mode, lid)
	p := path.Split(pn)
	fid, rest, err := pathc.resolve(p, uname, path.EndSlash(pn))
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "Error PutFile resolve %v %v %v: %v", pn, mode, lid, err)
		return 0, err
	}
	cnt, err := pathc.FidClnt.PutFile(fid, rest, mode, perm, off, data, path.EndSlash(pn), lid)
	if Retry(err) {
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
	} else if err != nil {
		return 0, err
	}
	return cnt, nil
}

func (pathc *PathClnt) resolve(p path.Path, uname sp.Tuname, resolve bool) (sp.Tfid, path.Path, *serr.Err) {
	if err, b := pathc.resolveRoot(p); err != nil {
		db.DPrintf(db.ALWAYS, "resolveRoot %v err %v b %v\n", p, err, b)
	}
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
