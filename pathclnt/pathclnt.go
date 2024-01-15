// The package pathclnt implements all path operations of the SigmaOS
// API. In particular it walks pathnames, mounting servers as needed,
// and allocates an fid for the file at the server.  All fid-based
// operations are inherited from [fidclnt].
//
// A typical use case is that each pathclnt has its own fidclnt.
// [sigmaclntd], however, shares a fidclnt among all its pathclnts,
// allows pathclnts to share a single TCP connection to a server.
package pathclnt

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type Watch func(error)

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
		fidc.NewClnt()
	}
	pathc.ndMntCache = NewNamedMountCache(pcfg)
	pathc.rootmt = newRootMountTable()
	pathc.cid = sp.TclntId(rand.Uint64())
	return pathc
}

func (pathc *PathClnt) String() string {
	str := fmt.Sprintf("Pathclnt cid %v mount table:\n", pathc.cid)
	str += fmt.Sprintf("%v\n", pathc.mnt)
	return str
}

func (pathc *PathClnt) Close() error {
	var err error
	if r := pathc.detachAll(); r != nil {
		db.DPrintf(db.TEST, "%v: detachall err %v\n", pathc.cid, r)
		err = r
	}
	if r := pathc.FidClnt.Close(); r != nil {
		db.DPrintf(db.TEST, "%v: close err %v\n", pathc.cid, r)
	}
	return err
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
		db.DPrintf(db.PATHCLNT_ERR, "%v: MountTree Attach [%v]/%v err %v", pathc.cid, addrs, tree, err)
		return err
	}
}

// Return path including the last mount file on this path and the rest
// of the path on the server.
func (pathc *PathClnt) PathLastMount(pn string, uname sp.Tuname) (path.Path, path.Path, error) {
	// Automount the longest prefix of pn; if pn exist, then the
	// server holding the directory/file correspending to pn.
	if _, err := pathc.Stat(pn+"/", uname); err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "%v: Stat %v err %v\n", pathc.cid, pn, err)
	}
	return pathc.LastMount(pn, uname)
}

// Detach from all servers
func (pathc *PathClnt) detachAll() error {
	var err error
	mnts := pathc.Mounts()
	db.DPrintf(db.ALWAYS, "%v: Fslib.detachAll %v\n", pathc.cid, mnts)
	for _, mnt := range mnts {
		if r := pathc.Detach(mnt); r != nil {
			db.DPrintf(db.TEST, "%v: detachAll %v err %v\n", pathc.cid, mnt, r)
			err = r
		}
	}
	return err
}

// Detach from server
func (pathc *PathClnt) Detach(pn string) error {
	fid, _, err := pathc.mnt.umount(path.Split(pn), true)
	if err != nil {
		db.DPrintf(db.TEST, "%v: Detach %q err %v\n", pathc.cid, pn, err)
		return err
	}
	defer pathc.FidClnt.Free(fid)
	if err := pathc.FidClnt.Detach(fid, pathc.cid); err != nil {
		return err
	}
	return nil
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
	db.DPrintf(db.PATHCLNT, "%v: Create %v perm %v lid %v\n", pathc.cid, p, perm, lid)
	path := path.Split(p)
	dir := path.Dir()
	base := path.Base()
	fid, err := pathc.walk(dir, uname, true, nil)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "%v: Walk failed: %v err %v", pathc.cid, p, err)
		return sp.NoFid, err
	}
	fid, err = pathc.FidClnt.Create(fid, base, perm, mode, lid, f)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "%v: create failed: %v err %v", pathc.cid, p, err)
		return sp.NoFid, err
	}
	return fid, nil
}

// Rename using renameat() for across directories or using wstat()
// for within a directory.
func (pathc *PathClnt) Rename(old, new string, uname sp.Tuname, f *sp.Tfence) error {
	db.DPrintf(db.PATHCLNT, "%v: Rename %v %v\n", pathc.cid, old, new)
	opath := path.Split(old)
	npath := path.Split(new)

	if len(opath) != len(npath) {
		if err := pathc.renameat(old, new, uname, f); err != nil {
			return err
		}
		return nil
	}
	for i, n := range opath[:len(opath)-1] {
		if npath[i] != n {
			if err := pathc.renameat(old, new, uname, f); err != nil {
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
	err = pathc.FidClnt.Wstat(fid, st, f)
	if err != nil {
		return err
	}
	return nil
}

// Rename across directories of a single server using Renameat
func (pathc *PathClnt) renameat(old, new string, uname sp.Tuname, f *sp.Tfence) *serr.Err {
	db.DPrintf(db.PATHCLNT, "%v: Renameat %v %v\n", pathc.cid, old, new)
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
	return pathc.FidClnt.Renameat(fid, o, fid1, n, f)
}

func (pathc *PathClnt) Remove(name string, uname sp.Tuname, f *sp.Tfence) error {
	db.DPrintf(db.PATHCLNT, "%v: Remove %v\n", pathc.cid, name)
	pn := path.Split(name)
	fid, rest, err := pathc.resolve(pn, uname, path.EndSlash(name))
	if err != nil {
		return err
	}
	err = pathc.FidClnt.RemoveFile(fid, rest, path.EndSlash(name), f)
	if Retry(err) {
		fid, err = pathc.walk(pn, uname, path.EndSlash(name), nil)
		if err != nil {
			return err
		}
		defer pathc.FidClnt.Clunk(fid)
		err = pathc.FidClnt.Remove(fid, f)
	} else if err != nil {
		return err
	}
	return nil
}

func (pathc *PathClnt) Stat(name string, uname sp.Tuname) (*sp.Stat, error) {
	db.DPrintf(db.PATHCLNT, "%v: Stat %v\n", pathc.cid, name)
	pn := path.Split(name)
	target, rest, err := pathc.resolve(pn, uname, true)
	if err != nil {
		db.DPrintf(db.ALWAYS, "%v: Stat resolve %v err %v\n", pathc.cid, pn, err)
	}
	db.DPrintf(db.PATHCLNT, "%v: Stat resolve %v target %v rest %v\n", pathc.cid, pn, target, rest)
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

func (pathc *PathClnt) Open(pn string, uname sp.Tuname, mode sp.Tmode, w Watch) (sp.Tfid, error) {
	db.DPrintf(db.PATHCLNT, "%v: Open %v %v %v\n", pathc.cid, pn, mode, w)
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

func (pathc *PathClnt) SetDirWatch(fid sp.Tfid, w Watch) error {
	db.DPrintf(db.PATHCLNT, "%v: SetDirWatch %v\n", pathc.cid, fid)
	go func() {
		err := pathc.FidClnt.Watch(fid)
		db.DPrintf(db.PATHCLNT, "%v: SetDirWatch: Watch returns %v %v\n", pathc.cid, fid, err)
		if err == nil {
			w(nil)
		} else {
			w(err)
		}
	}()
	return nil
}

func (pathc *PathClnt) SetRemoveWatch(pn string, uname sp.Tuname, w Watch) error {
	db.DPrintf(db.PATHCLNT, "%v: SetRemoveWatch %v", pathc.cid, pn)
	p := path.Split(pn)
	fid, err := pathc.walk(p, uname, path.EndSlash(pn), nil)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "%v: SetRemoveWatch: Walk %v err %v", pathc.cid, pn, err)
		return err
	}
	if w == nil {
		return serr.NewErr(serr.TErrInval, "watch")
	}
	go func() {
		err := pathc.FidClnt.Watch(fid)
		db.DPrintf(db.PATHCLNT, "%v: SetRemoveWatch: Watch %v %v err %v\n", pathc.cid, fid, pn, err)
		if err == nil {
			w(nil)
		} else {
			w(err)
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

func (pathc *PathClnt) GetFile(pn string, uname sp.Tuname, mode sp.Tmode, off sp.Toffset, cnt sp.Tsize, f *sp.Tfence) ([]byte, error) {
	db.DPrintf(db.PATHCLNT, "%v: GetFile %v %v\n", pathc.cid, pn, mode)
	p := path.Split(pn)
	fid, rest, err := pathc.resolve(p, uname, path.EndSlash(pn))
	if err != nil {
		return nil, err
	}
	data, err := pathc.FidClnt.GetFile(fid, rest, mode, off, cnt, path.EndSlash(pn), f)
	if Retry(err) {
		fid, err = pathc.walk(p, uname, path.EndSlash(pn), nil)
		if err != nil {
			return nil, err
		}
		defer pathc.FidClnt.Clunk(fid)
		data, err = pathc.FidClnt.GetFile(fid, []string{}, mode, off, cnt, false, f)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return data, nil
}

// Create or open file and write it
func (pathc *PathClnt) PutFile(pn string, uname sp.Tuname, mode sp.Tmode, perm sp.Tperm, data []byte, off sp.Toffset, lid sp.TleaseId, f *sp.Tfence) (sp.Tsize, error) {
	db.DPrintf(db.PATHCLNT, "%v: PutFile %v %v %v\n", pathc.cid, pn, mode, lid)
	p := path.Split(pn)
	fid, rest, err := pathc.resolve(p, uname, path.EndSlash(pn))
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "%v: Error PutFile resolve %v %v %v: %v", pathc.cid, pn, mode, lid, err)
		return 0, err
	}
	cnt, err := pathc.FidClnt.PutFile(fid, rest, mode, perm, off, data, path.EndSlash(pn), lid, f)
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
		cnt, err = pathc.FidClnt.PutFile(fid, base, mode, perm, off, data, false, lid, f)
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
		db.DPrintf(db.ALWAYS, "%v: resolveRoot %v err %v b %v\n", pathc.cid, p, err, b)
	}
	return pathc.mnt.resolve(p, resolve)
}

// XXX use MountedAt
func (pathc *PathClnt) LastMount(pn string, uname sp.Tuname) (path.Path, path.Path, error) {
	p := path.Split(pn)
	_, left, err := pathc.resolve(p, uname, path.EndSlash(pn))
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "%v: resolve  %v err %v\n", pathc.cid, pn, err)
		return nil, nil, err
	}
	p = p[0 : len(p)-len(left)]
	return p, left, nil
}

// Disconnect client from server permanently to simulate network
// partition to server that exports pn
func (pathc *PathClnt) Disconnect(pn string, fids []sp.Tfid) error {
	db.DPrintf(db.CRASH, "Disconnect %v mnts %v\n", pn, pathc.mnt.mountedPaths())
	mntp := pathc.mnt.mountedAt(path.Split(pn))
	for _, fid := range fids {
		ch := pathc.FidClnt.Lookup(fid)
		if ch != nil {
			p := path.Split(pn)
			if p.IsParent(ch.Path()) {
				db.DPrintf(db.CRASH, "fid disconnect fid %v %v %v\n", fid, ch, mntp)
				pathc.FidClnt.Disconnect(fid)
			}
		}
	}
	pathc.rootmt.disconnect(mntp.String())
	fid, err := pathc.mnt.disconnect(mntp)
	if err != nil {
		return err
	}
	pathc.FidClnt.Disconnect(fid)
	return nil
}
