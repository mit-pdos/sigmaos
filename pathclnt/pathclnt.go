package pathclnt

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/path"
	"sigmaos/reader"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/writer"
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
	chunkSz sessp.Tsize
	lip     string
}

func MakePathClnt(fidc *fidclnt.FidClnt, clntnet, lip string, sz sessp.Tsize) *PathClnt {
	pathc := &PathClnt{mnt: makeMntTable(), chunkSz: sz, lip: lip}
	if fidc == nil {
		pathc.FidClnt = fidclnt.MakeFidClnt(clntnet)
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

func (pathc *PathClnt) GetLocalIP() string {
	return pathc.lip
}

func (pathc *PathClnt) SetChunkSz(sz sessp.Tsize) {
	pathc.chunkSz = sz
}

func (pathc *PathClnt) GetChunkSz() sessp.Tsize {
	return pathc.chunkSz
}

func (pathc *PathClnt) Mounts() []string {
	return pathc.mnt.mountedPaths()
}

func (pathc *PathClnt) LastMount(pn string) (string, path.Path, error) {
	p := path.Split(pn)
	_, left, err := pathc.mnt.resolve(p, path.EndSlash(pn))
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "resolve  %v err %v\n", pn, err)
		return "", nil, err
	}
	p = p[0 : len(p)-len(left)]
	return p.String(), left, nil
}

// Exit the path client, closing all sessions
func (pathc *PathClnt) Exit() error {
	return pathc.FidClnt.Exit()
}

// Detach from server. XXX Mixes up umount a file system at server and
// closing session; if two mounts point to the same server; the first
// detach will close the session regardless of the second mount point.
func (pathc *PathClnt) Detach(pn string) error {
	fid, err := pathc.mnt.umount(path.Split(pn))
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Free(fid)
	if err := pathc.FidClnt.Detach(fid); err != nil {
		return err
	}
	return nil
}

// Simulate network partition to server that exports path
func (pathc *PathClnt) Disconnect(pn string) error {
	fid, err := pathc.mnt.umount(path.Split(pn))
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Free(fid)
	if err := pathc.FidClnt.Lookup(fid).Disconnect(); err != nil {
		return err
	}
	return nil
}

func (pathc *PathClnt) MakeReader(fid sp.Tfid, path string, chunksz sessp.Tsize) *reader.Reader {
	return reader.MakeReader(pathc.FidClnt, path, fid, chunksz)
}

func (pathc *PathClnt) MakeWriter(fid sp.Tfid) *writer.Writer {
	return writer.MakeWriter(pathc.FidClnt, fid)
}

func (pathc *PathClnt) readlink(fid sp.Tfid) ([]byte, *serr.Err) {
	qid := pathc.Qid(fid)
	if qid.Ttype()&sp.QTSYMLINK == 0 {
		return nil, serr.MkErr(serr.TErrNotSymlink, qid.Type)
	}
	_, err := pathc.FidClnt.Open(fid, sp.OREAD)
	if err != nil {
		return nil, err
	}
	rdr := reader.MakeReader(pathc.FidClnt, "", fid, pathc.chunkSz)
	b, err := rdr.GetDataErr()
	if err != nil {
		return nil, err
	}
	return b, nil
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

func (pathc *PathClnt) Create(p string, perm sp.Tperm, mode sp.Tmode) (sp.Tfid, error) {
	db.DPrintf(db.PATHCLNT, "Create %v perm %v\n", p, perm)
	path := path.Split(p)
	dir := path.Dir()
	base := path.Base()
	fid, err := pathc.WalkPath(dir, true, nil)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "Walk failed: %v", p)
		return sp.NoFid, err
	}
	fid, err = pathc.FidClnt.Create(fid, base, perm, mode)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "create failed: %v", p)
		return sp.NoFid, err
	}
	return fid, nil
}

func (pathc *PathClnt) ExerciseThree(pn string, mode sp.Tmode, data []byte) error {
	db.DPrintf(db.PATHCLNT, "Printing on server %v\n", data)
	p := path.Split(pn)
	fid, _, err := pathc.mnt.resolve(p, path.EndSlash(pn))
	if err != nil {
		return err
	}
	err = pathc.FidClnt.ExerciseThree(fid, data)
	return err
}

// Rename using renameat() for across directories or using wstat()
// for within a directory.
func (pathc *PathClnt) Rename(old string, new string) error {
	db.DPrintf(db.PATHCLNT, "Rename %v %v\n", old, new)
	opath := path.Split(old)
	npath := path.Split(new)

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
	fid, err := pathc.WalkPath(opath, path.EndSlash(old), nil)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Clunk(fid)
	st := sp.MkStatNull()
	st.Name = npath[len(npath)-1]
	err = pathc.FidClnt.Wstat(fid, st)
	if err != nil {
		return err
	}
	return nil
}

// Rename across directories of a single server using Renameat
func (pathc *PathClnt) renameat(old, new string) *serr.Err {
	db.DPrintf(db.PATHCLNT, "Renameat %v %v\n", old, new)
	opath := path.Split(old)
	npath := path.Split(new)
	o := opath[len(opath)-1]
	n := npath[len(npath)-1]
	fid, err := pathc.WalkPath(opath[:len(opath)-1], path.EndSlash(old), nil)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Clunk(fid)
	fid1, err := pathc.WalkPath(npath[:len(npath)-1], path.EndSlash(old), nil)
	if err != nil {
		return err
	}
	defer pathc.FidClnt.Clunk(fid1)
	return pathc.FidClnt.Renameat(fid, o, fid1, n)
}

func (pathc *PathClnt) umountFree(path []string) *serr.Err {
	if fid, err := pathc.mnt.umount(path); err != nil {
		return err
	} else {
		pathc.FidClnt.Free(fid)
		return nil
	}
}

func (pathc *PathClnt) Remove(name string) error {
	db.DPrintf(db.PATHCLNT, "Remove %v\n", name)
	pn := path.Split(name)
	fid, rest, err := pathc.mnt.resolve(pn, path.EndSlash(name))
	if err != nil {
		return err
	}

	// Optimistcally remove obj without doing a pathname
	// walk; this may fail if rest contains an automount
	// symlink.
	err = pathc.FidClnt.RemoveFile(fid, rest, path.EndSlash(name))
	if err != nil {
		if err.IsMaybeSpecialElem() || err.IsErrUnreachable() {
			fid, err = pathc.WalkPath(pn, path.EndSlash(name), nil)
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

func (pathc *PathClnt) Stat(name string) (*sp.Stat, error) {
	db.DPrintf(db.PATHCLNT, "Stat %v\n", name)
	pn := path.Split(name)
	// XXX ignore err?
	target, rest, _ := pathc.mnt.resolve(pn, true)
	if len(rest) == 0 && !path.EndSlash(name) {
		st := sp.MkStatNull()
		st.Name = pathc.FidClnt.Lookup(target).Servers().String()
		return st, nil
	} else {
		fid, err := pathc.WalkPath(path.Split(name), path.EndSlash(name), nil)
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

func (pathc *PathClnt) OpenWatch(pn string, mode sp.Tmode, w Watch) (sp.Tfid, error) {
	db.DPrintf(db.PATHCLNT, "Open %v %v\n", pn, mode)
	p := path.Split(pn)
	fid, err := pathc.WalkPath(p, path.EndSlash(pn), w)
	if err != nil {
		return sp.NoFid, err
	}
	_, err = pathc.FidClnt.Open(fid, mode)
	if err != nil {
		return sp.NoFid, err
	}
	return fid, nil
}

func (pathc *PathClnt) Open(path string, mode sp.Tmode) (sp.Tfid, error) {
	return pathc.OpenWatch(path, mode, nil)
}

func (pathc *PathClnt) SetDirWatch(fid sp.Tfid, path string, w Watch) error {
	db.DPrintf(db.PATHCLNT, "SetDirWatch %v\n", fid)
	go func() {
		err := pathc.FidClnt.Watch(fid)
		db.DPrintf(db.PATHCLNT, "SetDirWatch: Watch returns %v %v\n", fid, err)
		if err == nil {
			w(path, nil)
		} else {
			w(path, err)
		}
	}()
	return nil
}

func (pathc *PathClnt) SetRemoveWatch(pn string, w Watch) error {
	db.DPrintf(db.PATHCLNT, "SetRemoveWatch %v", pn)
	p := path.Split(pn)
	fid, err := pathc.WalkPath(p, path.EndSlash(pn), nil)
	if err != nil {
		return err
	}
	if w == nil {
		return serr.MkErr(serr.TErrInval, "watch")
	}
	go func() {
		err := pathc.FidClnt.Watch(fid)
		db.DPrintf(db.PATHCLNT, "SetRemoveWatch: Watch %v %v err %v\n", fid, pn, err)
		if err == nil {
			w(pn, nil)
		} else {
			w(pn, err)
		}
		pathc.Clunk(fid)
	}()
	return nil
}

func (pathc *PathClnt) GetFile(pn string, mode sp.Tmode, off sp.Toffset, cnt sessp.Tsize) ([]byte, error) {
	db.DPrintf(db.PATHCLNT, "GetFile %v %v\n", pn, mode)
	p := path.Split(pn)
	fid, rest, err := pathc.mnt.resolve(p, path.EndSlash(pn))
	if err != nil {
		return nil, err
	}
	// Optimistcally GetFile without doing a pathname
	// walk; this may fail if rest contains an automount
	// symlink.
	data, err := pathc.FidClnt.GetFile(fid, rest, mode, off, cnt, path.EndSlash(pn))
	if err != nil {
		if err.IsMaybeSpecialElem() {
			fid, err = pathc.WalkPath(p, path.EndSlash(pn), nil)
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

// Create or open file and write it
func (pathc *PathClnt) PutFile(pn string, mode sp.Tmode, perm sp.Tperm, data []byte, off sp.Toffset) (sessp.Tsize, error) {
	db.DPrintf(db.PATHCLNT, "PutFile %v %v\n", pn, mode)
	p := path.Split(pn)
	fid, rest, err := pathc.mnt.resolve(p, path.EndSlash(pn))
	if err != nil {
		return 0, err
	}
	// Optimistcally PutFile without doing a pathname
	// walk; this may fail if rest contains an automount
	// symlink.
	cnt, err := pathc.FidClnt.PutFile(fid, rest, mode, perm, off, data, path.EndSlash(pn))
	if err != nil {
		if err.IsMaybeSpecialElem() || err.IsErrUnreachable() {
			dir := p.Dir()
			base := path.Path{p.Base()}
			resolve := true
			if p.Base() == err.Obj { // was the final pn component a symlink?
				dir = p
				base = path.Path{}
				resolve = path.EndSlash(pn)
			}
			fid, err = pathc.WalkPath(dir, resolve, nil)
			if err != nil {
				return 0, err
			}
			defer pathc.FidClnt.Clunk(fid)
			cnt, err = pathc.FidClnt.PutFile(fid, base, mode, perm, off, data, false)
			if err != nil {
				return 0, err
			}
		} else {
			return 0, err
		}
	}
	return cnt, nil
}
