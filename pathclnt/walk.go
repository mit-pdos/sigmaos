package pathclnt

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

const (
	MAXSYMLINK = 8
)

func (pathc *PathClnt) Walk(fid sp.Tfid, path path.Tpathname, principal *sp.Tprincipal) (sp.Tfid, *serr.Err) {
	ch := pathc.FidClnt.Lookup(fid)
	if ch == nil {
		return sp.NoFid, serr.NewErr(serr.TErrNotfound, fid)
	}
	p := ch.Path().AppendPath(path)
	db.DPrintf(db.WALK, "Walk %v (ch %v)", p, ch.Path())
	return pathc.walk(p, principal, true, nil)
}

// WalkPath walks path and, on success, returns the fd walked to; it
// is the caller's responsibility to clunk the fd.  If a server is
// unreachable, it umounts the path it walked to, and starts over
// again, perhaps switching to another replica.  (Note:
// TestMaintainReplicationLevelCrashProcd test the fail-over case.)
func (pathc *PathClnt) walk(path path.Tpathname, principal *sp.Tprincipal, resolve bool, w Watch) (sp.Tfid, *serr.Err) {
	for i := 0; i < sp.PATHCLNT_MAXRETRY; i++ {
		if err, cont := pathc.mntclnt.ResolveRoot(path); err != nil {
			if cont && err.IsErrUnreachable() {
				db.DPrintf(db.MOUNT, "WalkPath: resolveRoot unreachable %v err %v\n", path, err)
				time.Sleep(sp.PATHCLNT_TIMEOUT * time.Millisecond)
				continue
			}
			db.DPrintf(db.MOUNT, "WalkPath: resolveRoot %v err %v\n", path, err)
			return sp.NoFid, err
		}
		start := time.Now()
		fid, path1, left, err := pathc.walkPath(path, resolve, w)
		db.DPrintf(db.WALK_LAT, "walkPath %v %v -> (%v, %v  %v, %v) lat: %v", pathc.cid, path, fid, path1, left, err, time.Since(start))
		if serr.Retry(err) {
			done := len(path1) - len(left)
			db.DPrintf(db.WALK_ERR, "Walk retry p %v %v l %v d %v err %v by umount %v\n", path, path1, left, done, err, path1[0:done])
			if e := pathc.mntclnt.UmountPrefix(path1[0:done]); e != nil {
				return sp.NoFid, e
			}
			// try again
			db.DPrintf(db.WALK_ERR, "walkPathUmount: retry p %v r %v\n", path, resolve)
			time.Sleep(sp.PATHCLNT_TIMEOUT * time.Millisecond)
			continue
		}
		if err != nil {
			return sp.NoFid, err
		}
		return fid, nil
	}
	return sp.NoFid, serr.NewErr(serr.TErrUnreachable, path)
}

// Walks path. If success, returns the fid for the path.  If failure,
// it returns NoFid and the rest of path that it wasn't able to walk.
// walkPath first walks the mount table, finding the server with the
// longest-match, and then uses walkOne() to walk at that server. The
// server may fail to walk, finish walking, or return the path element
// that is a union or symlink. In the latter case, walkPath() uses
// walkUnion() and walkSymlink to resolve that element. walkUnion()
// typically ends in a symlink.  walkSymlink will automount a new
// server and update the mount table. If succesfully automounted,
// walkPath() starts over again, but likely with a longer match in the
// mount table.  Each of the walk*() returns an fid, which on error is
// the same as the argument; and the caller is responsible for
// clunking it.
func (pathc *PathClnt) walkPath(path path.Tpathname, resolve bool, w Watch) (sp.Tfid, path.Tpathname, path.Tpathname, *serr.Err) {
	for i := 0; i < MAXSYMLINK; i++ {
		db.DPrintf(db.WALK, "walkPath: %v resolve %v\n", path, resolve)
		fid, left, err := pathc.walkMount(path, resolve)
		if err != nil {
			db.DPrintf(db.WALK, "walkPath: %v resolve %v\n", len(left), resolve)
			if len(left) != 0 || resolve {
				return sp.NoFid, path, left, err
			}
		}
		db.DPrintf(db.WALK, "walkPath: walkOne %v left %v\n", fid, left)
		fid, left, err = pathc.walkOne(fid, left, w)
		if err != nil {
			pathc.FidClnt.Clunk(fid)
			return sp.NoFid, path, left, err
		}

		retry, left, err := pathc.walkSymlink(fid, path, left, resolve)
		if err != nil {
			pathc.FidClnt.Clunk(fid)
			return sp.NoFid, path, left, err
		}
		db.DPrintf(db.WALK, "walkPath %v path/left %v retry %v err %v\n", fid, left, retry, err)
		if retry {
			// On success walkSymlink returns new path to walk
			path = left
			pathc.FidClnt.Clunk(fid)
			continue
		}

		fid, left, err = pathc.walkUnion(fid, left)
		if err != nil {
			pathc.FidClnt.Clunk(fid)
			return sp.NoFid, path, left, err
		}
		retry, left, err = pathc.walkSymlink(fid, path, left, resolve)
		if err != nil {
			pathc.FidClnt.Clunk(fid)
			return sp.NoFid, path, left, err
		}
		db.DPrintf(db.WALK, "walkPath %v path/left %v retry %v err %v\n", fid, left, retry, err)
		if retry {
			// On success walkSymlink returns new path to walk
			path = left
			pathc.FidClnt.Clunk(fid)
			continue
		}
		if len(left) == 0 {
			// Note: fid can be the one returned by walkMount
			return fid, path, nil, nil
		}
		return sp.NoFid, path, left, serr.NewErr(serr.TErrNotfound, left)
	}
	return sp.NoFid, path, path, serr.NewErr(serr.TErrUnreachable, "too many symlink cycles")
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
	db.DPrintf(db.WALK, "walkMount: resolve %v %v %v\n", fid, left, err)
	// Obtain a private copy of fid that this thread walks
	fid1, err := pathc.FidClnt.Clone(fid)
	if err != nil {
		return sp.NoFid, left, err
	}
	db.DPrintf(db.WALK_LAT, "walkMount: %v %v %v lat %v\n", pathc.cid, fid, left, time.Since(s))
	return fid1, left, nil
}

// Walk path at fid's server until the server runs into a symlink,
// union element, or an error. walkOne returns the fid walked too.  If
// file is not found, set watch on the directory, waiting until the
// file is created.
func (pathc *PathClnt) walkOne(fid sp.Tfid, path path.Tpathname, w Watch) (sp.Tfid, path.Tpathname, *serr.Err) {
	db.DPrintf(db.WALK, "walkOne %v left %v\n", fid, path)
	s := time.Now()
	fid1, left, err := pathc.FidClnt.Walk(fid, path)
	if err != nil { // fid1 == fid
		if w != nil && err.IsErrNotfound() {
			var err1 *serr.Err
			fid1, err1 = pathc.setWatch(fid, path, left, w)
			if err1 != nil {
				// couldn't walk to parent dir
				return fid, path, err1
			}
			if err1 == nil && fid1 == sp.NoFid {
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
		db.DFatalf("walkOne %v\n", fid)
	}
	db.DPrintf(db.WALK, "walkOne -> %v %v\n", fid1, left)
	err = pathc.FidClnt.Clunk(fid)
	db.DPrintf(db.WALK_LAT, "walkOne %v %v %v -> %v %v lat %v\n", pathc.cid, fid, path, fid1, left, time.Since(s))
	return fid1, left, nil
}

// Does fid point to a directory that contains ~?  If so, resolve ~
// and return fid for result.
func (pathc *PathClnt) walkUnion(fid sp.Tfid, p path.Tpathname) (sp.Tfid, path.Tpathname, *serr.Err) {
	if len(p) > 0 && path.IsUnionElem(p[0]) {
		db.DPrintf(db.WALK, "walkUnion %v path %v\n", fid, p)
		fid1, err := pathc.unionLookup(fid, p[0])
		if err != nil {
			return fid, p, err
		}
		db.DPrintf(db.WALK, "walkUnion -> (%v, %v)\n", fid, p[1:])
		pathc.FidClnt.Clunk(fid)
		return fid1, p[1:], nil
	}
	return fid, p, nil
}

// Is fid a symlink?  If so, walk it (incl. automounting) and return
// whether caller should retry.
func (pathc *PathClnt) walkSymlink(fid sp.Tfid, path, left path.Tpathname, resolve bool) (bool, path.Tpathname, *serr.Err) {
	qid := pathc.FidClnt.Lookup(fid).Lastqid()

	// if len(left) == 0 and !resolve, don't resolve
	// symlinks, so that the client can remove a symlink
	if qid.Ttype()&sp.QTSYMLINK == sp.QTSYMLINK && (len(left) > 0 || (len(left) == 0 && resolve)) {
		done := len(path) - len(left)
		resolved := path[0:done]
		db.DPrintf(db.WALK, "walkSymlink %v resolved %v left %v\n", fid, resolved, left)
		left, err := pathc.walkSymlink1(fid, resolved, left)
		if err != nil {
			return false, left, err
		}
		// start over again
		return true, left, nil
	}
	return false, left, nil
}

// Walk to parent directory, and check if name is there.  If it is,
// return entry.  Otherwise, set watch based on directory's version
// number. If the directory is modified between Walk and Watch(), the
// versions numbers won't match and Watch will return an error.
func (pathc *PathClnt) setWatch(fid sp.Tfid, p path.Tpathname, r path.Tpathname, w Watch) (sp.Tfid, *serr.Err) {
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
		db.DFatalf("setWatch %v %v\n", fid2, fid1)
	}
	go func() {
		err := pathc.FidClnt.Watch(fid1)
		pathc.FidClnt.Clunk(fid1)
		db.DPrintf(db.PATHCLNT, "setWatch: Watch returns %v %v\n", p, err)
		w(err)
	}()
	return sp.NoFid, nil
}
