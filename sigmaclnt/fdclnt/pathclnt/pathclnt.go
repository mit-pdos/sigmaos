// The package pathclnt implements all path operations of the SigmaOS
// API. In particular it walks pathnames, mounting servers as needed
// using [mntclnt], and allocates an fid for the file at the server.
// All fid-based operations are inherited from [fidclnt].
//
// A typical use case is that each pathclnt has its own fidclnt.
// [spproxyd], however, shares a fidclnt among all its pathclnts,
// allows pathclnts to share a single TCP connection to a server.
package pathclnt

import (
	"fmt"
	"path/filepath"

	sos "sigmaos/api/sigmaos"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fdclnt/pathclnt/mntclnt"
	"sigmaos/sigmaclnt/fidclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/rand"
)

type PathClnt struct {
	*fidclnt.FidClnt
	mntclnt      *mntclnt.MntClnt
	pe           *proc.ProcEnv
	cid          sp.TclntId
	disconnected bool // Used by test harness
}

func NewPathClnt(pe *proc.ProcEnv, fidc *fidclnt.FidClnt) *PathClnt {
	pathc := &PathClnt{
		pe:      pe,
		FidClnt: fidc,
		cid:     sp.TclntId(rand.Uint64()),
	}
	pathc.mntclnt = mntclnt.NewMntClnt(pathc, fidc, pathc.cid, pe, fidc.GetDialProxyClnt())
	db.DPrintf(db.TEST, "New cid %v %v\n", pathc.cid, pe.GetRealm())
	return pathc
}

func (pathc *PathClnt) String() string {
	return fmt.Sprintf("{Pathclnt: cid %v mount table %v fidclnt %v}", pathc.cid, pathc.mntclnt, pathc.FidClnt)
}

func (pathc *PathClnt) Close() error {
	db.DPrintf(db.PATHCLNT, "%v: Close", pathc.cid)
	var err error
	if r := pathc.detachAll(); r != nil {
		db.DPrintf(db.PATHCLNT_ERR, "%v: detachall err %v", pathc.cid, r)
		err = r
	}
	if r := pathc.FidClnt.Close(); r != nil {
		db.DPrintf(db.PATHCLNT_ERR, "%v: close err %v", pathc.cid, r)
	}
	return err
}

func (pathc *PathClnt) ClntId() sp.TclntId {
	return pathc.cid
}

func (pathc *PathClnt) MntClnt() *mntclnt.MntClnt {
	return pathc.mntclnt
}

// Detach from all servers
func (pathc *PathClnt) detachAll() error {
	var err error
	eps := pathc.mntclnt.MountedPaths()
	db.DPrintf(db.PATHCLNT, "%v: Fslib.detachAll %v\n", pathc.cid, eps)
	for _, ep := range eps {
		if r := pathc.mntclnt.Detach(ep); r != nil {
			db.DPrintf(db.PATHCLNT_ERR, "%v: detachAll %v err %v\n", pathc.cid, ep, r)
			err = r
		}
	}
	return err
}

func (pathc *PathClnt) Create(pn string, principal *sp.Tprincipal, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f *sp.Tfence) (sp.Tfid, error) {
	db.DPrintf(db.PATHCLNT, "%v: Create %v perm %v lid %v\n", pathc.cid, pn, perm, lid)
	if filepath.Base(pn) == sp.LOCAL || filepath.Base(pn) == sp.ANY {
		return sp.NoFid, fmt.Errorf("Can't create %v or %v: %v", sp.LOCAL, sp.ANY, pn)
	}
	path, err := serr.PathSplitErr(pn)
	if err != nil {
		return sp.NoFid, err
	}
	dir := path.Dir()
	base := path.Base()
	fid, err := pathc.walk(dir, principal, true, nil)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "%v: Walk failed: %v err %v", pathc.cid, pn, err)
		return sp.NoFid, err
	}
	fid, err = pathc.FidClnt.Create(fid, base, perm, mode, lid, f)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "%v: create failed: %v err %v", pathc.cid, pn, err)
		return sp.NoFid, err
	}
	return fid, nil
}

// Rename using renameat() for across directories or using wstat()
// for within a directory.
func (pathc *PathClnt) Rename(old, new string, principal *sp.Tprincipal, f *sp.Tfence) error {
	db.DPrintf(db.PATHCLNT, "%v: Rename %v %v\n", pathc.cid, old, new)
	opath, err := serr.PathSplitErr(old)
	if err != nil {
		return err
	}
	npath, err := serr.PathSplitErr(new)
	if err != nil {
		return err
	}
	if len(opath) != len(npath) {
		if err := pathc.renameat(old, new, principal, f); err != nil {
			return err
		}
		return nil
	}
	for i, n := range opath[:len(opath)-1] {
		if npath[i] != n {
			if err := pathc.renameat(old, new, principal, f); err != nil {
				return err
			}
			return nil
		}
	}
	fid, err := pathc.walk(opath, principal, path.EndSlash(old), nil)
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
func (pathc *PathClnt) renameat(old, new string, principal *sp.Tprincipal, f *sp.Tfence) *serr.Err {
	db.DPrintf(db.PATHCLNT, "%v: Renameat %v %v\n", pathc.cid, old, new)
	opath, err := serr.PathSplitErr(old)
	if err != nil {
		return err
	}
	npath, err := serr.PathSplitErr(new)
	if err != nil {
		return err
	}
	o := opath[len(opath)-1]
	n := npath[len(npath)-1]
	fid, err := pathc.walk(opath[:len(opath)-1], principal, path.EndSlash(old), nil)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Clunk(fid)
	fid1, err := pathc.walk(npath[:len(npath)-1], principal, path.EndSlash(old), nil)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Clunk(fid1)
	return pathc.FidClnt.Renameat(fid, o, fid1, n, f)
}

func (pathc *PathClnt) Remove(name string, principal *sp.Tprincipal, f *sp.Tfence) error {
	db.DPrintf(db.PATHCLNT, "%v: Remove %v\n", pathc.cid, name)

	pn, err := serr.PathSplitErr(name)
	if err != nil {
		return err
	}
	fid, rest, err := pathc.mntclnt.Resolve(pn, principal, path.EndSlash(name))
	if err != nil {
		return err
	}
	err = pathc.FidClnt.RemoveFile(fid, rest, path.EndSlash(name), f)
	if serr.Retry(err) {
		fid, err = pathc.walk(pn, principal, path.EndSlash(name), nil)
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

func (pathc *PathClnt) Stat(name string, principal *sp.Tprincipal) (*sp.Tstat, error) {
	db.DPrintf(db.PATHCLNT, "%v: Stat %v\n", pathc.cid, name)
	pn, err := serr.PathSplitErr(name)
	if err != nil {
		return nil, err
	}
	target, rest, err := pathc.mntclnt.Resolve(pn, principal, true)
	if err != nil {
		db.DPrintf(db.ALWAYS, "%v: Stat resolve %v err %v\n", pathc.cid, pn, err)
	}
	db.DPrintf(db.PATHCLNT, "%v: Stat resolve %v target %v rest %v\n", pathc.cid, pn, target, rest)
	if len(rest) == 0 && !path.EndSlash(name) {
		st := sp.NewStatNull()
		st.Name = pathc.FidClnt.Lookup(target).Endpoint().String()
		return st, nil
	} else {
		fid, err := pathc.walk(pn, principal, path.EndSlash(name), nil)
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

func (pathc *PathClnt) Open(pn string, principal *sp.Tprincipal, mode sp.Tmode, w sos.Watch) (sp.Tfid, error) {
	db.DPrintf(db.PATHCLNT, "%v: Open %v %v %v\n", pathc.cid, pn, mode, w)
	p, err := serr.PathSplitErr(pn)
	if err != nil {
		return sp.NoFid, err
	}
	fid, err := pathc.walk(p, principal, path.EndSlash(pn), w)
	if err != nil {
		return sp.NoFid, err
	}
	_, err = pathc.FidClnt.Open(fid, mode)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "%v: Open: open failed %v %v err %v\n", pathc.cid, fid, pn, err)
		return sp.NoFid, err
	}
	return fid, nil
}

func (pathc *PathClnt) SetDirWatch(fid sp.Tfid, w sos.Watch) error {
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

func (pathc *PathClnt) GetFile(pn string, principal *sp.Tprincipal, mode sp.Tmode, off sp.Toffset, cnt sp.Tsize, f *sp.Tfence) ([]byte, error) {
	db.DPrintf(db.PATHCLNT, "%v: GetFile %v %v\n", pathc.cid, pn, mode)
	p, err := serr.PathSplitErr(pn)
	if err != nil {
		return nil, err
	}
	fid, rest, err := pathc.mntclnt.Resolve(p, principal, path.EndSlash(pn))
	if err != nil {
		return nil, err
	}
	data, err := pathc.FidClnt.GetFile(fid, rest, mode, off, cnt, path.EndSlash(pn), f)
	if serr.Retry(err) {
		fid, err = pathc.walk(p, principal, path.EndSlash(pn), nil)
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
func (pathc *PathClnt) PutFile(pn string, principal *sp.Tprincipal, mode sp.Tmode, perm sp.Tperm, data []byte, off sp.Toffset, lid sp.TleaseId, f *sp.Tfence) (sp.Tsize, error) {
	db.DPrintf(db.PATHCLNT, "%v: PutFile %v %v %v\n", pathc.cid, pn, mode, lid)
	if filepath.Base(pn) == sp.LOCAL || filepath.Base(pn) == sp.ANY {
		return 0, fmt.Errorf("Can't create %v or %v: %v", sp.LOCAL, sp.ANY, pn)
	}
	p, err := serr.PathSplitErr(pn)
	if err != nil {
		return 0, err
	}
	fid, rest, err := pathc.mntclnt.Resolve(p, principal, path.EndSlash(pn))
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "%v: Error PutFile resolve %v %v %v: %v", pathc.cid, pn, mode, lid, err)
		return 0, err
	}
	cnt, err := pathc.FidClnt.PutFile(fid, rest, mode, perm, off, data, path.EndSlash(pn), lid, f)
	if serr.Retry(err) {
		dir := p.Dir()
		base := path.Tpathname{p.Base()}
		resolve := true
		if p.Base() == err.Obj { // was the final pn component a symlink?
			dir = p
			base = path.Tpathname{}
			resolve = path.EndSlash(pn)
		}
		fid, err = pathc.walk(dir, principal, resolve, nil)
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

// For npproxy
func (pathc *PathClnt) Walk(fid sp.Tfid, path path.Tpathname, principal *sp.Tprincipal) (sp.Tfid, *serr.Err) {
	ch := pathc.FidClnt.Lookup(fid)
	if ch == nil {
		return sp.NoFid, serr.NewErr(serr.TErrNotfound, fid)
	}

	// XXX fix
	// p := ch.Path().AppendPath(path)
	// return pathc.walk(p, principal, true, nil)

	db.DPrintf(db.PATHCLNT, "Walk %v %v (ch %v)", fid, path, ch)
	return pathc.walk(path, principal, true, nil)
}

func (pathc *PathClnt) Disconnected() bool {
	return pathc.disconnected
}

// Disconnect client from server permanently to simulate network
// partition to server that exports pn
func (pathc *PathClnt) Disconnect(pn string) error {
	db.DPrintf(db.CRASH, "Disconnect %q\n", pn)
	pathc.disconnected = true
	return pathc.mntclnt.Disconnect(pn)
}
