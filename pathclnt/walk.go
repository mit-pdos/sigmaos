package pathclnt

import (
	"log"

	db "ulambda/debug"
	np "ulambda/ninep"
)

const (
	MAXSYMLINK = 8
)

// walkManyUmount walks path and, on success, returns the fd walked
// to; it is the caller's responsibility to clunk the fd.  If it
// encounters an EOF err (e.g., server is not responding), it unmounts
// the path it walked to, and starts over again, perhaps switching to
// another replica.  (Note: TestMaintainReplicationLevelCrashProcd
// test the fail-over case.)
func (pathc *PathClnt) walkPathUmount(path []string, resolve bool, w Watch) (np.Tfid, *np.Err) {
	for {
		fid, left, err := pathc.walkPath(path, resolve, w)
		db.DLPrintf("WALK", "walkMany %v -> (%v, %v)\n", path, fid, err)
		if err != nil && np.IsErrEOF(err) {
			done := len(path) - len(left)
			db.DLPrintf("WALK", "walkManyUmount: umount %v\n", path[0:done])
			if e := pathc.mnt.umount(pathc.FidClnt, path[0:done]); e != nil {
				return np.NoFid, e
			}
			// try again
			db.DLPrintf("WALK", "walkManyUmount: try again p %v r %v\n", path, resolve)
			continue
		}
		if err != nil {
			return np.NoFid, err
		}
		return fid, nil
	}
}

// Walks path. If success, returns the fid for the path.  If failure,
// it returns NoFid and the rest of path that it wasn't able to walk.
// walkPath first walks the mount table, finding the server with the
// longest-match, and then uses walkOne() to walk at that server. The
// server may fail to resolve, succeed resolving, or return the path
// element that is a union or symlink. In the latter case, walkMany()
// uses walkUnion() and walkSymlink to resolve that
// element. walkUnion() typically ends in a symlink.  walkSymlink will
// automount a new server and update the mount table. If succesful,
// walkMany() starts over again, but likely with a longer match in the
// mount table.  Each of the walk*() returns an fid, which on error is
// the same as the argument; and the caller is responsible for
// clunking it.
func (pathc *PathClnt) walkPath(path []string, resolve bool, w Watch) (np.Tfid, []string, *np.Err) {
	for i := 0; i < MAXSYMLINK; i++ {
		fid, left, err := pathc.walkMount(path)
		if err != nil {
			return np.NoFid, left, err
		}
		fid, left, err = pathc.walkOne(fid, left, w)
		if err != nil {
			pathc.FidClnt.Clunk(fid)
			return np.NoFid, left, err
		}
		fid, left, err = pathc.walkUnion(fid, left)
		if err != nil {
			pathc.FidClnt.Clunk(fid)
			return np.NoFid, left, err
		}
		retry, left, err := pathc.walkSymlink(fid, path, left, resolve)
		if err != nil {
			pathc.FidClnt.Clunk(fid)
			return np.NoFid, path, err
		}
		db.DLPrintf("WALK", "walkPath %v path/left %v retry %v err %v\n", fid, left, retry, err)
		if retry {
			// On success walkSymlink returns new path to walk
			path = left
			pathc.FidClnt.Clunk(fid)
			continue
		}
		if len(left) == 0 {
			// Note: fid can be the one returned by walkMount
			return fid, nil, nil
		}
		return np.NoFid, left, np.MkErr(np.TErrNotfound, left)
	}
	return np.NoFid, path, np.MkErr(np.TErrNotfound, "too many symlink cycles")
}

// Walk the mount table, and clone the found fid; the caller is
// responsible for clunking it. Return the fid and the remaining part
// of the path that must be walked.
func (pathc *PathClnt) walkMount(path []string) (np.Tfid, []string, *np.Err) {
	fid, left, err := pathc.mnt.resolve(path)
	if err != nil {
		return np.NoFid, left, err
	}
	db.DLPrintf("WALK", "walkMount: resolve %v %v %v\n", fid, left, err)
	// Clone starting point
	fid1, _, err := pathc.FidClnt.Walk(fid, nil)
	if err != nil {
		return np.NoFid, left, err
	}
	return fid1, left, nil
}

// Walk path at fid's server until the server runs into a symlink,
// union element, or an error. walkOne returns the fid walked too.
func (pathc *PathClnt) walkOne(fid np.Tfid, path []string, w Watch) (np.Tfid, []string, *np.Err) {
	db.DLPrintf("WALK", "walkOne %v left %v\n", fid, path)
	fid1, left, err := pathc.FidClnt.Walk(fid, path)
	if err != nil { // fid1 == fid
		if w != nil && np.IsErrNotfound(err) {
			var err1 *np.Err
			fid1, err1 = pathc.setWatch(fid, path, left, w)
			if err1 != nil {
				// couldn't walk to parent dir
				return fid, path, err1
			}
			if err1 == nil && fid1 == np.NoFid {
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
		log.Fatalf("FATAL walkOne %v\n", fid)
	}
	db.DLPrintf("WALK", "walkOne -> %v %v\n", fid1, left)
	err = pathc.FidClnt.Clunk(fid)
	return fid1, left, nil
}

// Does fid point to a directory that contains ~?  If, resolve union
// and return fid for result.
func (pathc *PathClnt) walkUnion(fid np.Tfid, path []string) (np.Tfid, []string, *np.Err) {
	if len(path) > 0 && np.IsUnionElem(path[0]) {
		db.DLPrintf("WALK", "walkUnion %v path %v\n", fid, path)
		fid1, err := pathc.unionLookup(fid, path[0])
		if err != nil {
			return fid, path, err
		}
		db.DLPrintf("WALK", "walkUnion -> (%v, %v)\n", fid, path[1:])
		pathc.FidClnt.Clunk(fid)
		return fid1, path[1:], nil
	}
	return fid, path, nil
}

// Is fid a symlink?  If so, walk it (incl. automounting) and return
// whether caller should retry.
func (pathc *PathClnt) walkSymlink(fid np.Tfid, path, left []string, resolve bool) (bool, []string, *np.Err) {
	qid := pathc.FidClnt.Lookup(fid).Lastqid()

	// if len(left) == 0 and !resolve, don't resolve
	// symlinks, so that the client can remove a symlink
	if qid.Type&np.QTSYMLINK == np.QTSYMLINK && (len(left) > 0 || (len(left) == 0 && resolve)) {
		done := len(path) - len(left)
		resolved := path[0:done]
		db.DLPrintf("WALK", "walkSymlink %v resolved %v left %v\n", fid, resolved, left)
		left, err := pathc.walkSymlink1(fid, resolved, left)
		if err != nil {
			return false, left, err
		}
		// start over again
		return true, left, nil
	}
	return false, left, nil
}

// Walk to parent directory, and check if name is there.  If it is, return entry.
// Otherwise, set watch based on directory's version number
func (pathc *PathClnt) setWatch(fid np.Tfid, p []string, r []string, w Watch) (np.Tfid, *np.Err) {
	fid1, _, err := pathc.FidClnt.Walk(fid, np.Dir(r))
	if err != nil {
		return np.NoFid, err
	}
	fid2, _, err := pathc.FidClnt.Walk(fid1, []string{np.Base(r)})
	if err == nil {
		pathc.FidClnt.Clunk(fid1)
		return fid2, nil
	}
	if fid2 != fid1 { // Walk returns fd where it stops
		log.Fatalf("FATAL setWatch %v %v\n", fid2, fid1)
	}
	go func(version np.TQversion) {
		err := pathc.FidClnt.Watch(fid1, np.Dir(r), version)
		pathc.FidClnt.Clunk(fid1)
		db.DLPrintf("WALK", "setWatch: Watch returns %v %v\n", p, err)
		w(np.Join(p), err)
	}(pathc.FidClnt.Lookup(fid1).Version())
	return np.NoFid, nil
}
