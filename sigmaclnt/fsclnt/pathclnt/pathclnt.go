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
	"time"

	sos "sigmaos/api/sigmaos"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fidclnt"
	"sigmaos/sigmaclnt/fsclnt/pathclnt/mntclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/rand"
	"sigmaos/util/retry"
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

func (pathc *PathClnt) Create(pn sp.Tsigmapath, principal *sp.Tprincipal, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f *sp.Tfence) (sp.Tfid, error) {
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
	fid, err := pathc.open(dir, principal, true, nil)
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
func (pathc *PathClnt) Rename(old, new sp.Tsigmapath, principal *sp.Tprincipal, f *sp.Tfence) error {
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
	fid, err := pathc.open(opath, principal, path.EndSlash(old), nil)
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
func (pathc *PathClnt) renameat(old, new sp.Tsigmapath, principal *sp.Tprincipal, f *sp.Tfence) *serr.Err {
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
	fid, err := pathc.open(opath[:len(opath)-1], principal, path.EndSlash(old), nil)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Clunk(fid)
	fid1, err := pathc.open(npath[:len(npath)-1], principal, path.EndSlash(old), nil)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Clunk(fid1)
	return pathc.FidClnt.Renameat(fid, o, fid1, n, f)
}

func (pathc *PathClnt) Remove(pn sp.Tsigmapath, principal *sp.Tprincipal, f *sp.Tfence) error {
	db.DPrintf(db.PATHCLNT, "%v: Remove %v\n", pathc.cid, pn)
	splitPN, err := serr.PathSplitErr(pn)
	if err != nil {
		return err
	}
	open := false
	fid, rest, err := pathc.mntclnt.Resolve(splitPN, principal, path.EndSlash(pn))
	if err == nil {
		err = pathc.FidClnt.RemoveFile(fid, rest, path.EndSlash(pn), f)
	} else {
		open = true
	}
	if open || serr.IsErrorWalkOK(err) {
		fid, err = pathc.open(splitPN, principal, path.EndSlash(pn), nil)
		if err != nil {
			return err
		}
		// remove will also clunk fid
		err = pathc.FidClnt.Remove(fid, f)
	} else if err != nil {
		return err
	}
	return nil
}

func (pathc *PathClnt) Stat(pn sp.Tsigmapath, principal *sp.Tprincipal) (*sp.Tstat, error) {
	db.DPrintf(db.PATHCLNT, "%v: Stat %v\n", pathc.cid, pn)
	splitPN, err := serr.PathSplitErr(pn)
	if err != nil {
		return nil, err
	}
	target, rest, err := pathc.mntclnt.Resolve(splitPN, principal, true)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "%v: Stat resolve %v err %v\n", pathc.cid, splitPN, err)
	}
	db.DPrintf(db.PATHCLNT, "%v: Stat resolve %v target %v rest %v\n", pathc.cid, splitPN, target, rest)
	if len(rest) == 0 && !path.EndSlash(pn) {
		st := sp.NewStatNull()
		st.Name = pathc.FidClnt.Lookup(target).Endpoint().String()
		return st, nil
	} else {
		fid, err := pathc.open(splitPN, principal, path.EndSlash(pn), nil)
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

// open walks path and, on success, returns the fd walked to; it is
// the caller's responsibility to clunk the fd.  If a server is
// unreachable, it umounts the path it walked to, and starts over
// again, perhaps switching to another replica or recovered server.
func (pathc *PathClnt) open(path path.Tpathname, principal *sp.Tprincipal, resolve bool, w sos.Watch) (sp.Tfid, *serr.Err) {
	fid := sp.NoFid
	err, ok := retry.RetryDefDurCont(func() (error, bool) {
		if err, cont := pathc.mntclnt.ResolveRoot(path); err != nil {
			db.DPrintf(db.PATHCLNT_ERR, "open: resolveRoot %v err %v cont %t", path, err, cont)
			return err, cont
		}
		start := time.Now()
		fid0, path1, left, err := pathc.walkPath(path, resolve, w)
		db.DPrintf(db.WALK_LAT, "open %v %v -> (%v, %v  %v, %v) lat: %v", pathc.cid, path, fid, path1, left, err, time.Since(start))
		if serr.IsErrorRetryOpenOK(err) {
			done := len(path1) - len(left)
			db.DPrintf(db.PATHCLNT_ERR, "walkPath retry pn '%v' pn1 '%v' left '%v' d %v err %v by umount %v", path, path1, left, done, err, path1[0:done])
			if e := pathc.mntclnt.UmountPrefix(path1[0:done]); e != nil {
				return e, true
			}
			db.DPrintf(db.PATHCLNT_ERR, "open: retry pn '%v' r %v", path, resolve)
		}
		if err != nil {
			return err, true
		}
		fid = fid0
		return nil, true
	}, serr.IsErrorRetryOpenOK)
	if !ok {
		return sp.NoFid, serr.NewErr(serr.TErrUnreachable, path)
	}
	if err != nil {
		return sp.NoFid, err.(*serr.Err)
	}
	return fid, nil
}

func (pathc *PathClnt) Open(pn sp.Tsigmapath, principal *sp.Tprincipal, mode sp.Tmode, w sos.Watch) (sp.Tfid, error) {
	db.DPrintf(db.PATHCLNT, "%v: Open %v %v %v\n", pathc.cid, pn, mode, w)
	p, err := serr.PathSplitErr(pn)
	if err != nil {
		return sp.NoFid, err
	}
	fid, err := pathc.open(p, principal, path.EndSlash(pn), w)
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

func (pathc *PathClnt) SetDirWatch(fid sp.Tfid) (sp.Tfid, error) {
	db.DPrintf(db.PATHCLNT, "%v: SetDirWatch %v\n", pathc.cid, fid)
	watchfid, err := pathc.FidClnt.Watch(fid)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "%v: SetDirWatch: setting watch failed %v err %v\n", pathc.cid, fid, err)
		return sp.NoFid, err
	}
	return watchfid, nil
}

func (pathc *PathClnt) GetFile(pn sp.Tsigmapath, principal *sp.Tprincipal, mode sp.Tmode, off sp.Toffset, cnt sp.Tsize, f *sp.Tfence) ([]byte, error) {
	db.DPrintf(db.PATHCLNT, "%v: GetFile %v %v\n", pathc.cid, pn, mode)
	p, err := serr.PathSplitErr(pn)
	if err != nil {
		return nil, err
	}
	open := false
	var data []byte
	fid, rest, err := pathc.mntclnt.Resolve(p, principal, path.EndSlash(pn))
	if err == nil {
		data, err = pathc.FidClnt.GetFile(fid, rest, mode, off, cnt, path.EndSlash(pn), f)
	} else {
		open = true
	}
	if open || serr.IsErrorWalkOK(err) {
		fid, err = pathc.open(p, principal, path.EndSlash(pn), nil)
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
func (pathc *PathClnt) PutFile(pn sp.Tsigmapath, principal *sp.Tprincipal, mode sp.Tmode, perm sp.Tperm, data []byte, off sp.Toffset, lid sp.TleaseId, f *sp.Tfence) (sp.Tsize, error) {
	db.DPrintf(db.PATHCLNT, "%v: PutFile %v %v %v\n", pathc.cid, pn, mode, lid)
	if filepath.Base(pn) == sp.LOCAL || filepath.Base(pn) == sp.ANY {
		return 0, fmt.Errorf("Can't create %v or %v: %v", sp.LOCAL, sp.ANY, pn)
	}
	p, err := serr.PathSplitErr(pn)
	if err != nil {
		return 0, err
	}
	open := false
	cnt := sp.Tsize(0)
	fid, rest, err := pathc.mntclnt.Resolve(p, principal, path.EndSlash(pn))
	if err == nil {
		cnt, err = pathc.FidClnt.PutFile(fid, rest, mode, perm, off, data, path.EndSlash(pn), lid, f)
	} else {
		open = true
	}
	if open || serr.IsErrorWalkOK(err) {
		dir := p.Dir()
		base := path.Tpathname{p.Base()}
		resolve := true
		if p.Base() == err.Obj { // was the final pn component a symlink?
			dir = p
			base = path.Tpathname{}
			resolve = path.EndSlash(pn)
		}
		fid, err = pathc.open(dir, principal, resolve, nil)
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

// For ninep proxy
func (pathc *PathClnt) Walk(fid1 sp.Tfid, path path.Tpathname, principal *sp.Tprincipal) (sp.Tfid, *serr.Err) {
	// XXX fix
	// p := ch.Path().AppendPath(path)
	// return pathc.walk(p, principal, true, nil)

	// Obtain a private copy of fid that this thread walks, which
	// walkPathFid closes.
	fid, err := pathc.FidClnt.Clone(fid1)
	if err != nil {
		return sp.NoFid, err
	}
	fid, left, retry, err := pathc.walkPathFid1(fid, path, path, true, nil)

	db.DPrintf(db.NPPROXY, "Walk: walkPathFid %v path '%v'  fid %v left '%v' r %t err %v", fid1, path, fid, left, retry, err)

	return fid, err
}

func (pathc *PathClnt) Disconnected() bool {
	return pathc.disconnected
}

// Disconnect client from server permanently to simulate network
// partition to server that exports pn
func (pathc *PathClnt) Disconnect(pn sp.Tsigmapath) error {
	db.DPrintf(db.CRASH, "Disconnect %q\n", pn)
	pathc.disconnected = true
	return pathc.mntclnt.Disconnect(pn)
}
