package pathclnt

import (
	"fmt"
	"strings"

	db "ulambda/debug"
	"ulambda/fidclnt"
	np "ulambda/ninep"
	"ulambda/reader"
	"ulambda/writer"
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
	chunkSz np.Tsize
}

func MakePathClnt(fidc *fidclnt.FidClnt, sz np.Tsize) *PathClnt {
	pathc := &PathClnt{}
	pathc.mnt = makeMntTable()
	pathc.chunkSz = sz
	if fidc == nil {
		pathc.FidClnt = fidclnt.MakeFidClnt()
	} else {
		pathc.FidClnt = fidc
	}
	return pathc
}

func (pathc *PathClnt) String() string {
	str := fmt.Sprintf("Pathclnt mount table:\n")
	str += fmt.Sprintf("%v\n", pathc.mnt)
	return str
}

func (pathc *PathClnt) SetChunkSz(sz np.Tsize) {
	pathc.chunkSz = sz
}

func (pathc *PathClnt) GetChunkSz() np.Tsize {
	return pathc.chunkSz
}

// Close the path client, umounting any mounted file system and
// closing session to them.  XXX not implemented
func (pathc *PathClnt) Shutdown() error {
	return pathc.mnt.close()
}

// Simulate network partition to server that exports path
func (pathc *PathClnt) Disconnect(path string) error {
	fid, err := pathc.mnt.umount(np.Split(path))
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Free(fid)
	if err := pathc.FidClnt.Lookup(fid).Disconnect(); err != nil {
		return err
	}
	return nil
}

func (pathc *PathClnt) MakeReader(fid np.Tfid, path string, chunksz np.Tsize) *reader.Reader {
	return reader.MakeReader(pathc.FidClnt, path, fid, chunksz)
}

func (pathc *PathClnt) MakeWriter(fid np.Tfid, chunksz np.Tsize) *writer.Writer {
	return writer.MakeWriter(pathc.FidClnt, fid, chunksz)
}

func (pathc *PathClnt) readlink(fid np.Tfid) (string, *np.Err) {
	qid := pathc.Qid(fid)
	if qid.Type&np.QTSYMLINK == 0 {
		return "", np.MkErr(np.TErrNotSymlink, qid.Type)
	}
	_, err := pathc.FidClnt.Open(fid, np.OREAD)
	if err != nil {
		return "", err
	}
	rdr := reader.MakeReader(pathc.FidClnt, "", fid, pathc.chunkSz)
	b, err := rdr.GetDataErr()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (pathc *PathClnt) mount(fid np.Tfid, path string) *np.Err {
	if err := pathc.mnt.add(np.Split(path), fid); err != nil {
		if err.Code() == np.TErrExists {
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

func (pathc *PathClnt) Mount(fid np.Tfid, path string) error {
	if err := pathc.mount(fid, path); err != nil {
		return err
	}
	return nil
}

func (pathc *PathClnt) Create(p string, perm np.Tperm, mode np.Tmode) (np.Tfid, error) {
	db.DPrintf("PATHCLNT", "Create %v perm %v\n", p, perm)
	path := np.Split(p)
	dir := path.Dir()
	base := path.Base()
	fid, err := pathc.walkPathUmount(dir, true, nil)
	if err != nil {
		return np.NoFid, err
	}
	fid, err = pathc.FidClnt.Create(fid, base, perm, mode)
	if err != nil {
		return np.NoFid, err
	}
	return fid, nil
}

// Rename using renameat() for across directories or using wstat()
// for within a directory.
func (pathc *PathClnt) Rename(old string, new string) error {
	db.DPrintf("PATHCLNT", "Rename %v %v\n", old, new)
	opath := np.Split(old)
	npath := np.Split(new)

	if len(opath) != len(npath) {
		if err := pathc.renameat(old, new); err != nil {
			return err
		}
		return nil
	}
	for i, n := range opath[:len(opath)-1] {
		if npath[i] != n {
			if err := pathc.renameat(old, new); err != nil {
				return err
			}
			return nil
		}
	}
	fid, err := pathc.walkPathUmount(opath, np.EndSlash(old), nil)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Clunk(fid)
	st := &np.Stat{}
	st.Name = npath[len(npath)-1]
	err = pathc.FidClnt.Wstat(fid, st)
	if err != nil {
		return err
	}
	return nil
}

// Rename across directories of a single server using Renameat
func (pathc *PathClnt) renameat(old, new string) *np.Err {
	db.DPrintf("PATHCLNT", "Renameat %v %v\n", old, new)
	opath := np.Split(old)
	npath := np.Split(new)
	o := opath[len(opath)-1]
	n := npath[len(npath)-1]
	fid, err := pathc.walkPathUmount(opath[:len(opath)-1], np.EndSlash(old), nil)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Clunk(fid)
	fid1, err := pathc.walkPathUmount(npath[:len(npath)-1], np.EndSlash(old), nil)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Clunk(fid1)
	return pathc.FidClnt.Renameat(fid, o, fid1, n)
}

func (pathc *PathClnt) umountFree(path []string) *np.Err {
	if fid, err := pathc.mnt.umount(path); err != nil {
		return err
	} else {
		pathc.FidClnt.Free(fid)
		return nil
	}
}

func (pathc *PathClnt) Umount(path []string) error {
	db.DPrintf("PATHCLNT", "Umount %v\n", path)
	return pathc.umountFree(path)
}

func (pathc *PathClnt) Remove(name string) error {
	db.DPrintf("PATHCLNT", "Remove %v\n", name)
	path := np.Split(name)
	fid, rest, err := pathc.mnt.resolve(path)
	if err != nil {
		return err
	}
	// Optimistcally remove obj without doing a pathname
	// walk; this may fail if rest contains an automount
	// symlink.
	err = pathc.FidClnt.RemoveFile(fid, rest, np.EndSlash(name))
	if err != nil {
		if np.IsMaybeSpecialElem(err) {
			fid, err = pathc.walkPathUmount(path, np.EndSlash(name), nil)
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

func (pathc *PathClnt) Stat(name string) (*np.Stat, error) {
	db.DPrintf("PATHCLNT", "Stat %v\n", name)
	path := np.Split(name)
	// XXX ignore err?
	target, rest, _ := pathc.mnt.resolve(path)
	if len(rest) == 0 && !np.EndSlash(name) {
		st := &np.Stat{}
		st.Name = strings.Join(pathc.FidClnt.Lookup(target).Server(), ",")
		return st, nil
	} else {
		fid, err := pathc.walkPathUmount(np.Split(name), np.EndSlash(name), nil)
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

func (pathc *PathClnt) OpenWatch(path string, mode np.Tmode, w Watch) (np.Tfid, error) {
	db.DPrintf("PATHCLNT", "Open %v %v\n", path, mode)
	p := np.Split(path)
	fid, err := pathc.walkPathUmount(p, np.EndSlash(path), w)
	if err != nil {
		return np.NoFid, err
	}
	_, err = pathc.FidClnt.Open(fid, mode)
	if err != nil {
		return np.NoFid, err
	}
	return fid, nil
}

func (pathc *PathClnt) Open(path string, mode np.Tmode) (np.Tfid, error) {
	return pathc.OpenWatch(path, mode, nil)
}

func (pathc *PathClnt) SetDirWatch(path string, w Watch) error {
	db.DPrintf("PATHCLNT", "SetDirWatch %v\n", path)
	p := np.Split(path)
	fid, err := pathc.walkPathUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return err
	}
	go func() {
		err := pathc.FidClnt.Watch(fid)
		db.DPrintf("PATHCLNT", "SetDirWatch: Watch returns %v %v\n", path, err)
		if err == nil {
			w(path, nil)
		} else {
			w(path, err)
		}
		pathc.Clunk(fid)
	}()
	return nil
}

func (pathc *PathClnt) SetRemoveWatch(path string, w Watch) error {
	db.DPrintf("PATHCLNT", "SetRemoveWatch %v\n", path)
	p := np.Split(path)
	fid, err := pathc.walkPathUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return err
	}
	if w == nil {
		return np.MkErr(np.TErrInval, "watch")
	}
	go func() {
		err := pathc.FidClnt.Watch(fid)
		db.DPrintf("PATHCLNT", "SetRemoveWatch: Watch %v %v err %v\n", fid, path, err)
		if err == nil {
			w(path, nil)
		} else {
			w(path, err)
		}
		pathc.Clunk(fid)
	}()
	return nil
}

func (pathc *PathClnt) GetFile(path string, mode np.Tmode, off np.Toffset, cnt np.Tsize) ([]byte, error) {
	db.DPrintf("PATHCLNT", "GetFile %v %v\n", path, mode)
	p := np.Split(path)
	fid, rest, err := pathc.mnt.resolve(p)
	if err != nil {
		return nil, err
	}
	// Optimistcally GetFile without doing a pathname
	// walk; this may fail if rest contains an automount
	// symlink.
	data, err := pathc.FidClnt.GetFile(fid, rest, mode, off, cnt, np.EndSlash(path))
	if err != nil {
		if np.IsMaybeSpecialElem(err) {
			fid, err = pathc.walkPathUmount(p, np.EndSlash(path), nil)
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

// Write file
func (pathc *PathClnt) SetFile(path string, mode np.Tmode, data []byte, off np.Toffset) (np.Tsize, error) {
	db.DPrintf("PATHCLNT", "SetFile %v %v\n", path, mode)
	p := np.Split(path)
	fid, rest, err := pathc.mnt.resolve(p)
	if err != nil {
		return 0, err
	}
	// Optimistcally SetFile without doing a pathname walk; this
	// may fail if rest contains an automount symlink.
	// XXX On EOF try another replica for TestMaintainReplicationLevelCrashProcd
	cnt, err := pathc.FidClnt.SetFile(fid, rest, mode, off, data, np.EndSlash(path))
	if err != nil {
		if np.IsMaybeSpecialElem(err) || np.IsErrUnreachable(err) {
			fid, err = pathc.walkPathUmount(p, np.EndSlash(path), nil)
			if err != nil {
				return 0, err
			}
			defer pathc.FidClnt.Clunk(fid)
			cnt, err = pathc.FidClnt.SetFile(fid, []string{}, mode, off, data, false)
			if err != nil {
				return 0, err
			}
		} else {
			return 0, err
		}
	}
	return cnt, nil
}

// Create file
func (pathc *PathClnt) PutFile(path string, mode np.Tmode, perm np.Tperm, data []byte, off np.Toffset) (np.Tsize, error) {
	db.DPrintf("PATHCLNT", "PutFile %v %v\n", path, mode)
	p := np.Split(path)
	fid, rest, err := pathc.mnt.resolve(p)
	if err != nil {
		return 0, err
	}
	// Optimistcally PutFile without doing a pathname
	// walk; this may fail if rest contains an automount
	// symlink.
	cnt, err := pathc.FidClnt.PutFile(fid, rest, mode, perm, off, data)
	if err != nil {
		if np.IsMaybeSpecialElem(err) || np.IsErrUnreachable(err) {
			dir := p.Dir()
			base := np.Path{p.Base()}
			fid, err = pathc.walkPathUmount(dir, true, nil)
			if err != nil {
				return 0, err
			}
			defer pathc.FidClnt.Clunk(fid)
			cnt, err = pathc.FidClnt.PutFile(fid, base, mode, perm, off, data)
			if err != nil {
				return 0, err
			}
		} else {
			return 0, err
		}
	}
	return cnt, nil
}

// Return path to the root directory for last server on path
func (pathc *PathClnt) PathServer(path string) (string, error) {
	if _, err := pathc.Stat(path + "/"); err != nil {
		db.DPrintf("PATHCLNT_ERR", "PathServer: stat %v err %v\n", path, err)
		return "", err
	}
	p := np.Split(path)
	_, left, err := pathc.mnt.resolve(p)
	if err != nil {
		db.DPrintf("PATHCLNT_ERR", "resolve  %v err %v\n", path, err)
		return "", err
	}
	p = p[0 : len(p)-len(left)]
	return p.String(), nil
}
