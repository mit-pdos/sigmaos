package pathclnt

import (
	"log"

	np "ulambda/ninep"
)

const (
	MAXSYMLINK = 8
)

// walkManyUmount walks p using walkMany, but if it returns an EOF err
// (e.g., server is not responding), it unmounts the server and starts
// over again, perhaps switching to another replica.
func (pathc *PathClnt) walkManyUmount(path []string, resolve bool, w Watch) (np.Tfid, *np.Err) {
	for {
		fid, err := pathc.walkMany(path, resolve, w)
		if err != nil && np.IsErrEOF(err) {
			channel := pathc.FidClnt.Lookup(fid)
			// XXX schedd doesn't exist anymore; fix?
			if channel == nil { // schedd triggers this; don't know why
				return np.NoFid, err
			}
			if len(channel.Path()) == 0 { // nothing to umount
				return fid, err
			}
			fid2, e := pathc.mnt.umount(channel.Path())
			if e != nil {
				return fid, e
			}
			pathc.FidClnt.Free(fid2)
			pathc.FidClnt.Clone(fid)
			// try again
			continue
		}
		if err != nil {
			return fid, err
		}
		return fid, nil
	}
}

// Walks path.  It uses walkOne() to walk on the server that has the
// longest-match in the mount.  That server may fail or succeed
// resolving the path, or return at the path element that is a union
// or symlink. In the latter case, walkMany() uses walkUnion() and
// walkSymlink to resolve that element. walkUnion() typically ends in
// a symlink. So in both cases, walkSymlink will automount a new
// server and update the mount table. If succesful, walkMany() starts
// over again, but likely with a longer match in the mount table.
func (pathc *PathClnt) walkMany(path []string, resolve bool, w Watch) (np.Tfid, *np.Err) {
	for i := 0; i < MAXSYMLINK; i++ {
		fid, left, err := pathc.walkOne(path, w)
		if err != nil {
			return fid, err
		}

		// fid maybe points to symlink or directory that contains ~
		if len(left) > 0 && np.IsUnionElem(left[0]) {
			fid, left, err = pathc.walkUnion(fid, left)
			if err != nil {
				log.Printf("walk union %v %v err %v\n", pathc.FidClnt.Lookup(fid), path, err)
				return fid, err
			}
			// this may have produced a symlink qid
		}
		qid := pathc.FidClnt.Lookup(fid).Lastqid()

		// if todo == 0 and !resolve, don't resolve symlinks, so
		// that the client can remove a symlink
		if qid.Type&np.QTSYMLINK == np.QTSYMLINK && (len(left) > 0 ||
			(len(left) == 0 && resolve)) {
			resolved := len(path) - len(left)
			path, err = pathc.walkSymlink(fid, path[0:resolved], left)
			if err != nil {
				log.Printf("walk link %v %v err %v\n", pathc.FidClnt.Lookup(fid), path, err)
				return fid, err
			}
			// start over again
			pathc.FidClnt.Clunk(fid)
			continue
		}
		return fid, err
	}
	return np.NoFid, np.MkErr(np.TErrNotfound, "too many symlink cycles")
}

// Walk to parent directory, and check if name is there.  If it is, return entry.
// Otherwise, set watch based on directory's version number
func (pathc *PathClnt) setWatch(fid1 np.Tfid, p []string, r []string, w Watch) (np.Tfid, *np.Err) {
	fid3, _, err := pathc.FidClnt.Walk(fid1, np.Dir(r))
	if err != nil {
		return np.NoFid, err
	}
	fid2, _, err := pathc.FidClnt.Walk(fid3, []string{np.Base(r)})
	if err == nil {
		return fid2, nil
	}
	go func(version np.TQversion) {
		err := pathc.FidClnt.Watch(fid3, np.Dir(r), version)
		pathc.FidClnt.Clunk(fid3)
		w(np.Join(p), err)
	}(pathc.FidClnt.Lookup(fid3).Version())
	return np.NoFid, nil
}

// Resolves path until it runs into a symlink, union element, or an
// error.
func (pathc *PathClnt) walkOne(path []string, w Watch) (np.Tfid, []string, *np.Err) {
	fid, rest := pathc.mnt.resolve(path)
	if fid == np.NoFid {
		if pathc.mnt.hasExited() {
			return np.NoFid, path, np.MkErr(np.TErrEOF, "mount")
		}
		return np.NoFid, path, np.MkErr(np.TErrNotfound, "mount")
	}
	fid2, left, err := pathc.FidClnt.Walk(fid, rest)
	if err != nil {
		if w != nil && np.IsErrNotfound(err) {
			var err1 *np.Err
			fid2, err1 = pathc.setWatch(fid, path, rest, w)
			if err1 != nil {
				// couldn't walk to parent dir
				return np.NoFid, path, err1
			}
			if err1 == nil && fid2 == np.NoFid {
				// entry is still not in parent dir
				return np.NoFid, path, err
			}
			left = nil
			// entry now exists
		} else {
			return np.NoFid, path, err
		}
	}
	return fid2, left, nil
}
