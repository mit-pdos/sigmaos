package pathclnt

import (
	"time"

	sos "sigmaos/api/sigmaos"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

// Walk path. If success, returns the fid for the path.  If failure,
// it returns NoFid and the rest of path that it wasn't able to walk.
// walkPath first walks the mount table, finding the server with the
// longest-match, and then uses walkPathFid() to walk at that server.
// If retry, walkPathFid() returned a new path to be walked (e.g.,
// when walkPathFid traversed a symbolic link or endpoint that also
// may have updated the mount table).
func (pathc *PathClnt) walkPath(path path.Tpathname, resolve bool, w sos.Watch) (sp.Tfid, path.Tpathname, path.Tpathname, *serr.Err) {
	for i := 0; i < sp.Conf.Path.MAX_SYMLINK; i++ {
		db.DPrintf(db.WALK, "walkPath: '%v' resolve %v", path, resolve)
		fid, left, err := pathc.walkMount(path, resolve)
		if err != nil {
			db.DPrintf(db.WALK, "walkPath: left '%v' resolve %v err %v", left, resolve, err)
			if len(left) != 0 || resolve {
				return sp.NoFid, path, left, err
			}
		}
		retry := false
		fid, left, retry, err = pathc.walkPathFid(fid, path, left, resolve, w)
		if err != nil {
			db.DPrintf(db.WALK_ERR, "walkPath: walkPathFid %v path '%v' left '%v' retry %t err %v(%T)", fid, path, left, retry, err, err)
			return sp.NoFid, path, left, err
		}
		if retry {
			db.DPrintf(db.WALK, "walkPath: retry %v path '%v' left '%v'", fid, path, left)
			path = left
			continue
		}
		db.DPrintf(db.WALK, "walkPath: done %v path '%v' left '%v'", fid, path, left)
		return fid, path, left, nil
	}
	return sp.NoFid, path, path, serr.NewErr(serr.TErrUnreachable, "too many symlink cycles")
}

// Walk path at server identified by and starting from fid. walkOne
// may fail to walk, finish walking, or return the path element that
// is a union or symlink. In the latter case, walkPathFid() uses
// walkUnion() and walkSymlink to resolve that element. walkUnion()
// typically ends in a symlink.  walkSymlink will automount a new
// server and update the mount table and/or return a new path to walk
// (if retry). Each of the walk*() returns an fid, which on error is
// the same as the argument; and the caller is responsible for
// clunking it.
func (pathc *PathClnt) walkPathFid(fid1 sp.Tfid, path, left path.Tpathname, resolve bool, w sos.Watch) (sp.Tfid, path.Tpathname, bool, *serr.Err) {
	db.DPrintf(db.WALK, "walkPathFid: walkOne %v left '%v'", fid1, path)
	fid, left, err := pathc.walkOne(fid1, left, w)
	if err != nil {
		pathc.FidClnt.Clunk(fid)
		return sp.NoFid, left, false, err
	}
	retry, left, err := pathc.walkSymlink(fid, path, left, resolve)
	if err != nil {
		pathc.FidClnt.Clunk(fid)
		return sp.NoFid, left, false, err
	}
	db.DPrintf(db.WALK, "walkPathFid %v left '%v' retry %v err %v", fid, left, retry, err)
	if retry {
		// On success walkSymlink returns new path to walk
		pathc.FidClnt.Clunk(fid)
		return sp.NoFid, left, true, nil

	}
	fid, left, err = pathc.walkUnion(fid, left)
	if err != nil {
		pathc.FidClnt.Clunk(fid)
		return sp.NoFid, left, false, err
	}
	retry, left, err = pathc.walkSymlink(fid, path, left, resolve)
	if err != nil {
		pathc.FidClnt.Clunk(fid)
		return sp.NoFid, left, false, err
	}
	db.DPrintf(db.WALK, "walkPathFid %v left '%v' retry %v err %v", fid, left, retry, err)
	if retry {
		// On success walkSymlink returns new path to walk
		pathc.FidClnt.Clunk(fid)
		return sp.NoFid, left, true, nil
	}
	if len(left) == 0 {
		// Note: fid can be the one returned by walkMount
		return fid, nil, false, nil
	}
	return sp.NoFid, left, false, serr.NewErr(serr.TErrNotfound, left)
}

func (pathc *PathClnt) walkPathFid1(fid1 sp.Tfid, path, left path.Tpathname, resolve bool, w sos.Watch) (sp.Tfid, path.Tpathname, bool, *serr.Err) {
	db.DPrintf(db.WALK, "walkPathFid: walkOne %v left '%v'", fid1, path)
	fid, left, err := pathc.walkOne(fid1, left, w)
	if err != nil {
		pathc.FidClnt.Clunk(fid)
		return sp.NoFid, left, false, err
	}
	retry, fid2, left, err := pathc.walkSymlink1(fid, path, left, resolve)
	if err != nil {
		pathc.FidClnt.Clunk(fid)
		return sp.NoFid, left, false, err
	}
	db.DPrintf(db.WALK, "walkPathFid %v fid2 %v left '%v' retry %v err %v", fid, fid2, left, retry, err)
	if retry {
		// On success walkSymlink returns new path to walk
		pathc.FidClnt.Clunk(fid)
		return sp.NoFid, left, true, nil

	}
	if fid2 != sp.NoFid {
		// endpoint
		return fid2, left, false, err
	}
	fid, left, err = pathc.walkUnion(fid, left)
	if err != nil {
		pathc.FidClnt.Clunk(fid)
		return sp.NoFid, left, false, err
	}
	retry, fid2, left, err = pathc.walkSymlink1(fid, path, left, resolve)
	if err != nil {
		pathc.FidClnt.Clunk(fid)
		return sp.NoFid, left, false, err
	}
	db.DPrintf(db.WALK, "walkPathFid %v left '%v' retry %v err %v", fid, left, retry, err)
	if retry {
		// On success walkSymlink returns new path to walk
		pathc.FidClnt.Clunk(fid)
		return sp.NoFid, left, true, nil
	}
	if len(left) == 0 {
		// Note: fid can be the one returned by walkMount
		return fid, nil, false, nil
	}
	return sp.NoFid, left, false, serr.NewErr(serr.TErrNotfound, left)
}

// Walk the mount table, and clone the found fid; the caller is
// responsible for clunking it. Return the fid and the remaining part
// of the path that must be walked.
func (pathc *PathClnt) walkMount(path path.Tpathname, resolve bool) (sp.Tfid, path.Tpathname, *serr.Err) {
	s := time.Now()
	fid, left, err := pathc.mntclnt.ResolveMnt(path, resolve)
	if err != nil {
		return sp.NoFid, left, err
	}
	db.DPrintf(db.WALK, "walkMount[%v]: resolve %v %v %v", path, fid, left, err)
	s1 := time.Now()
	// Obtain a private copy of fid that this thread walks
	fid1, err := pathc.FidClnt.Clone(fid)
	if err != nil {
		return sp.NoFid, left, err
	}
	db.DPrintf(db.WALK_LAT, "walkMount [%v]: cid %v fid %v left %v lat %v (clone lat %v)", path, pathc.cid, fid, left, time.Since(s), time.Since(s1))
	return fid1, left, nil
}

// Walk path at fid's server until the server runs into a symlink,
// union element, or an error. walkOne returns the fid walked too.  If
// file is not found, set watch on the directory, waiting until the
// file is created.
func (pathc *PathClnt) walkOne(fid sp.Tfid, path path.Tpathname, w sos.Watch) (sp.Tfid, path.Tpathname, *serr.Err) {
	db.DPrintf(db.WALK, "walkOne %v left %v", fid, path)
	s := time.Now()
	fid1, left, err := pathc.FidClnt.Walk(fid, path)
	if err != nil { // fid1 == fid
		db.DPrintf(db.WALK, "walkOne %v left %v err %v", fid, path, err)
		if w != nil && err.IsErrNotfound() {
			var err1 *serr.Err
			fid1, err1 = pathc.setWatch(fid, path, left, w)
			if err1 != nil {
				db.DPrintf(db.WALK, "walkOne %v left %v couldn't walk to parent dir err1 %v", fid, path, err1)
				// couldn't walk to parent dir
				return fid, path, err1
			}
			if err1 == nil && fid1 == sp.NoFid {
				db.DPrintf(db.WALK, "walkOne %v left %v entry still not in parent dir", fid, path)
				// entry is still not in parent dir
				return fid, path, err
			}
			left = nil
			// entry now exists
		} else {
			return fid, path, err
		}
	}
	if fid1 == fid {
		db.DFatalf("walkOne %v", fid)
	}
	db.DPrintf(db.WALK, "walkOne -> %v %v", fid1, left)
	s1 := time.Now()
	pathc.FidClnt.Clunk(fid)
	db.DPrintf(db.WALK_LAT, "walkOne cid %v fid %v path %v -> fid1 %v left %v lat %v (clunk lat %v)", pathc.cid, fid, path, fid1, left, time.Since(s), time.Since(s1))
	return fid1, left, nil
}

// Does fid point to a directory that contains ~?  If so, resolve ~
// and return fid for result.
func (pathc *PathClnt) walkUnion(fid sp.Tfid, p path.Tpathname) (sp.Tfid, path.Tpathname, *serr.Err) {
	if len(p) > 0 && path.IsUnionElem(p[0]) {
		if p[0] == sp.LOCAL && pathc.pe.GetKernelID() != sp.NOT_SET {
			start := time.Now()
			fid1, err := pathc.unionScan(fid, pathc.pe.GetKernelID(), sp.LOCAL)
			if err != nil {
				db.DPrintf(db.TEST, "Local unionScan fid %v err %v", fid, err)
				return fid, p, err
			}
			db.DPrintf(db.WALK_LAT, "walkUnion/unionScan: KernelId %v path %v time %v", pathc.pe.GetKernelID(), p, time.Since(start))
			return fid1, p[1:], nil
		}
		s := time.Now()
		db.DPrintf(db.WALK, "walkUnion %v path %v", fid, p)
		fid1, err := pathc.unionLookup(fid, p[0])
		if err != nil {
			return fid, p, err
		}
		db.DPrintf(db.WALK, "walkUnion -> (%v, %v)", fid, p[1:])
		pathc.FidClnt.Clunk(fid)
		db.DPrintf(db.WALK_LAT, "walkUnion %v lat %v", p, time.Since(s))
		return fid1, p[1:], nil
	}
	return fid, p, nil
}

// Is fid a symlink?  If so, walk it (incl. automounting) and return
// whether caller should retry.
func (pathc *PathClnt) walkSymlink(fid sp.Tfid, path, left path.Tpathname, resolve bool) (bool, path.Tpathname, *serr.Err) {
	db.DPrintf(db.WALK, "walkSymlink %v path %v left %v resolve %v", fid, path, left, resolve)
	qid := pathc.FidClnt.Lookup(fid).Lastqid()

	// if len(left) == 0 and !resolve, don't resolve
	// symlinks, so that the client can remove a symlink
	if qid.Ttype()&sp.QTSYMLINK == sp.QTSYMLINK && (len(left) > 0 || (len(left) == 0 && resolve)) {
		done := len(path) - len(left)
		resolved := path[0:done]
		db.DPrintf(db.WALK, "walkSymlink %v resolved %v left %v", fid, resolved, left)
		left, err := pathc.walkReadSymlink(fid, resolved, left)
		if err != nil {
			return false, left, err
		}
		// start over again
		return true, left, nil
	}
	return false, left, nil
}

// Is fid a symlink?  If so, walk it (incl. automounting) and return
// whether caller should retry.
func (pathc *PathClnt) walkSymlink1(fid sp.Tfid, path, left path.Tpathname, resolve bool) (bool, sp.Tfid, path.Tpathname, *serr.Err) {
	db.DPrintf(db.WALK, "walkSymlink1 %v path %v left '%v' resolve %v", fid, path, left, resolve)
	qid := pathc.FidClnt.Lookup(fid).Lastqid()

	// if len(left) == 0 and !resolve, don't resolve
	// symlinks, so that the client can remove a symlink
	if qid.Ttype()&sp.QTSYMLINK == sp.QTSYMLINK && (len(left) > 0 || (len(left) == 0 && resolve)) {
		done := len(path) - len(left)
		resolved := path[0:done]
		fid1, pn, err := pathc.walkReadSymlink1(fid, resolved)
		db.DPrintf(db.WALK, "walkReadSymlink1 %v fid1 %v pn '%v' resolved %v left '%v'", fid, fid1, pn, resolved, left)
		if err != nil {
			return false, sp.NoFid, left, err
		}
		if pn != nil { // symbolic link
			pn = append(pn, left...)
			// start over again
			return true, sp.NoFid, pn, nil
		}

		return false, fid1, left, nil
	}
	return false, sp.NoFid, left, nil
}

// Walk to parent directory, and check if name is there.  If it is,
// return entry.  Otherwise, set watch based on directory's version
// number. If the directory is modified between Walk and Watch(), the
// versions numbers won't match and Watch will return an error.
func (pathc *PathClnt) setWatch(fid sp.Tfid, p path.Tpathname, r path.Tpathname, w sos.Watch) (sp.Tfid, *serr.Err) {
	fid1, _, err := pathc.FidClnt.Walk(fid, r.Dir())
	if err != nil {
		return sp.NoFid, err
	}
	fid2, _, err := pathc.FidClnt.Walk(fid1, path.Tpathname{r.Base()})
	if err == nil {
		pathc.FidClnt.Clunk(fid1)
		return fid2, nil
	}
	if fid2 != fid1 { // Walk returns fd where it stops
		db.DFatalf("setWatch %v %v", fid2, fid1)
	}
	go func() {
		var err error
		watchFid, err1 := pathc.FidClnt.Watch(fid1)
		if err1 == nil {
			b := make([]byte, 1)
			_, err = pathc.FidClnt.ReadF(watchFid, 0, b, sp.NullFence())
		} else {
			err = err1.Unwrap()
		}

		pathc.FidClnt.Clunk(fid1)
		if err1 != nil {
			pathc.FidClnt.Clunk(watchFid)
		}
		db.DPrintf(db.PATHCLNT, "setWatch: Watch returns %v %v", p, err)
		w(err)
	}()
	return sp.NoFid, nil
}
