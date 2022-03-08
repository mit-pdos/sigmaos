package pathclnt

import (
	"fmt"
	"log"
	"strings"

	db "ulambda/debug"
	"ulambda/fidclnt"
	np "ulambda/ninep"
	"ulambda/proc"
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
	p := np.Split(path)
	fid, _, err := pathc.mnt.resolve(p)
	if err != nil {
		return err
	}
	pathc.FidClnt.Lookup(fid).Disconnect()
	return nil
}

func (pathc *PathClnt) MakeReader(fid np.Tfid, path string, chunksz np.Tsize) *reader.Reader {
	return reader.MakeReader(pathc.FidClnt, path, fid, chunksz)
}

func (pathc *PathClnt) MakeWriter(fid np.Tfid, chunksz np.Tsize) *writer.Writer {
	return writer.MakeWriter(pathc.FidClnt, fid, chunksz)
}

func (pathc *PathClnt) readlink(fid np.Tfid) (string, *np.Err) {
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
	db.DLPrintf("PATHCLNT", "Create %v perm %v\n", p, perm)
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
	db.DLPrintf("PATHCLNT", "Rename %v %v\n", old, new)
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
	db.DLPrintf("PATHCLNT", "Renameat %v %v\n", old, new)
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

func (pathc *PathClnt) Umount(path []string) error {
	db.DLPrintf("PATHCLNT", "Umount %v\n", path)
	if err := pathc.mnt.umount(pathc.FidClnt, path); err != nil {
		return err
	}
	return nil
}

func (pathc *PathClnt) Remove(name string) error {
	db.DLPrintf("PATHCLNT", "Remove %v\n", name)
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
	db.DLPrintf("PATHCLNT", "Stat %v\n", name)
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
	db.DLPrintf("PATHCLNT", "Open %v %v\n", path, mode)
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
	db.DLPrintf("PATHCLNT", "SetDirWatch %v\n", path)
	p := np.Split(path)
	fid, err := pathc.walkPathUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return err
	}
	go func() {
		version := pathc.FidClnt.Lookup(fid).Version()
		err := pathc.FidClnt.Watch(fid, nil, version)
		db.DLPrintf("PATHCLNT", "SetDirWatch: Watch returns %v %v\n", path, err)
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
	db.DLPrintf("PATHCLNT", "SetRemoveWatch %v\n", path)
	p := np.Split(path)
	fid, err := pathc.walkPathUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return err
	}
	if w == nil {
		return np.MkErr(np.TErrInval, "watch")
	}
	go func() {
		version := pathc.FidClnt.Lookup(fid).Version()
		err := pathc.FidClnt.Watch(fid, nil, version)
		db.DLPrintf("PATHCLNT", "SetRemoveWatch: Watch returns %v %v\n", path, err)
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
	db.DLPrintf("PATHCLNT", "GetFile %v %v\n", path, mode)
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
	db.DLPrintf("PATHCLNT", "SetFile %v %v\n", path, mode)
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
	db.DLPrintf("PATHCLNT", "PutFile %v %v\n", path, mode)
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

func (pathc *PathClnt) MakeFence(path string, mode np.Tmode) (np.Tfence, error) {
	db.DLPrintf("PATHCLNT", "MakeFence %v %v\n", path, mode)
	p := np.Split(path)
	fid, err := pathc.walkPathUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return np.Tfence{}, err
	}
	defer pathc.FidClnt.Clunk(fid)
	_, err = pathc.FidClnt.Open(fid, mode)
	if err != nil {
		return np.Tfence{}, err
	}
	fence, err := pathc.FidClnt.MkFence(fid)
	if err != nil {
		log.Printf("%v: MkFence %v err %v\n", proc.GetProgram(), fid, err)
		return np.Tfence{}, err
	}
	return fence, nil
}

func (pathc *PathClnt) RegisterFence(f np.Tfence, path string) error {
	db.DLPrintf("PATHCLNT", "RegisterFence %v %v\n", f, path)
	p := np.Split(path)
	fid, err := pathc.walkPathUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Clunk(fid)
	if err := pathc.FidClnt.RegisterFence(f, fid); err != nil {
		return err
	}
	return nil
}

func (pathc *PathClnt) DeregisterFence(f np.Tfence, path string) error {
	db.DLPrintf("PATHCLNT", "DeregisterFence %v %v\n", f, path)
	p := np.Split(path)
	fid, err := pathc.walkPathUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Clunk(fid)
	if err := pathc.FidClnt.DeregisterFence(f, fid); err != nil {
		return err
	}
	return nil
}

func (pathc *PathClnt) RmFence(f np.Tfence, path string) error {
	db.DLPrintf("PATHCLNT", "RmFence %v %v\n", f, path)
	p := np.Split(path)
	fid, err := pathc.walkPathUmount(p, np.EndSlash(path), nil)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Clunk(fid)
	if err := pathc.FidClnt.RmFence(f, fid); err != nil {
		return err
	}
	return nil
}
