package fsclnt

import (
	"log"

	np "ulambda/ninep"
	"ulambda/protclnt"
)

const (
	MAXSYMLINK = 8
)

// walkManyUmount walks p using walkMany, but if it returns an EOF err
// (e.g., server is not responding), it unmounts the server and starts
// over again, perhaps switching to another replica.
func (fsc *FsClient) walkManyUmount(p []string, resolve bool, w Watch) (np.Tfid, *np.Err) {
	var fid np.Tfid
	for {
		f, err := fsc.walkMany(p, resolve, w)
		if err != nil && np.IsErrEOF(err) {
			p := fsc.fids.path(f)
			// XXX schedd doesn't exist anymore; fix?
			if p == nil { // schedd triggers this; don't know why
				return np.NoFid, err
			}
			if len(p.cname) == 0 { // nothing to umount
				return f, err
			}
			fid2, e := fsc.mnt.umount(p.cname)
			if e != nil {
				return f, e
			}
			fsc.fids.freeFid(fid2)
			continue
		}
		if err != nil {
			return f, err
		}
		fid = f
		break
	}
	return fid, nil
}

// Walks path.  It uses walkOne() to walk on the server that has the
// longest-match in the mount.  That server may fail or succeed
// resolving the path, or return at the path element that is a union
// or symlink. In the latter case, walkMany() uses walkUnion() and
// walkSymlink to resolve that element. walkUnion() typically ends in
// a symlink. So in both cases, walkSymlink will automount a new
// server and update the mount table. If succesful, walkMany() starts
// over again, but likely with a longer match in the mount table.
// XXX clunking fid?
func (fsc *FsClient) walkMany(path []string, resolve bool, w Watch) (np.Tfid, *np.Err) {
	for i := 0; i < MAXSYMLINK; i++ {
		fid, todo, err := fsc.walkOne(path, w)
		if err != nil {
			return fid, err
		}

		// fid maybe points to symlink or directory that contains ~
		i := len(path) - todo
		if todo > 0 && np.IsUnionElem(path[i]) {
			fid, todo, err = fsc.walkUnion(fid, path, todo)
			if err != nil {
				log.Printf("walk union %v %v err %v\n", fsc.fids.lookup(fid), path, err)
				return fid, err
			}
			// this may have produced a symlink qid
		}
		qid := fsc.fids.path(fid).lastqid()

		// if todo == 0 and !resolve, don't resolve symlinks, so
		// that the client can remove a symlink
		if qid.Type&np.QTSYMLINK == np.QTSYMLINK && (todo > 0 ||
			(todo == 0 && resolve)) {
			path, err = fsc.walkSymlink(fid, path, todo)
			if err != nil {
				log.Printf("walk link %v %v err %v\n", fsc.fids.lookup(fid), path, err)
				return fid, err
			}
			continue
		}
		return fid, err
	}
	return np.NoFid, np.MkErr(np.TErrNotfound, "too many symlink cycles")
}

// Walk to parent directory, and check if name is there.  If it is, return entry.
// Otherwise, set watch based on directory's version number
func (fsc *FsClient) setWatch(fid1, fid2 np.Tfid, p []string, r []string, w Watch) (*np.Rwalk, *np.Err) {
	fid3 := fsc.fids.allocFid()
	dir := r[0 : len(r)-1]
	reply, err := fsc.fids.clnt(fid1).Walk(fid1, fid3, dir)
	if err != nil {
		return nil, err
	}
	fsc.fids.addFid(fid3, fsc.fids.path(fid1).copyPath())
	fsc.fids.path(fid3).addn(reply.Qids, dir)

	reply, err = fsc.fids.clnt(fid3).Walk(fid3, fid2, []string{r[len(r)-1]})
	if err == nil {
		return reply, nil
	}

	go func(pc *protclnt.ProtClnt, version np.TQversion) {
		err := pc.Watch(fid3, []string{r[len(r)-1]}, version)
		fsc.clunkFid(fid3)
		w(np.Join(p), err)
	}(fsc.fids.clnt(fid3), fsc.fids.path(fid3).lastqid().Version)
	return nil, nil
}

// Resolves path until it runs into a symlink, union element, or an
// error.
func (fsc *FsClient) walkOne(path []string, w Watch) (np.Tfid, int, *np.Err) {
	fid, rest := fsc.mnt.resolve(path)
	if fid == np.NoFid {
		if fsc.mnt.hasExited() {
			return np.NoFid, 0, np.MkErr(np.TErrEOF, "mount")
		}
		return np.NoFid, 0, np.MkErr(np.TErrNotfound, "mount")
	}
	fid1, err := fsc.clone(fid)
	if err != nil {
		return fid, 0, err
	}
	defer fsc.clunkFid(fid1)
	fid2 := fsc.fids.allocFid()
	reply, err := fsc.fids.clnt(fid1).Walk(fid1, fid2, rest)
	if err != nil {
		if w != nil && np.IsErrNotfound(err) {
			var err1 *np.Err
			reply, err1 = fsc.setWatch(fid1, fid2, path, rest, w)
			if err1 != nil {
				// couldn't walk to parent dir
				return np.NoFid, 0, err1
			}
			if err1 == nil && reply == nil {
				// entry is still not in parent dir
				return np.NoFid, 0, err
			}
			// entry now exists
		} else {
			return np.NoFid, 0, err
		}
	}
	todo := len(rest) - len(reply.Qids)
	if len(reply.Qids) == 0 {
		fid2, err = fsc.clone(fid1)
		if err != nil {
			return np.NoFid, 0, err
		}
	} else {
		fsc.fids.addFid(fid2, fsc.fids.path(fid1).copyPath())
		fsc.fids.path(fid2).addn(reply.Qids, rest)
	}
	return fid2, todo, nil
}
