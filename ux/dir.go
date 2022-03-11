package fsux

import (
	"errors"
	"io/ioutil"
	"os"
	"sort"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
)

type Dir struct {
	*Obj
}

func makeDir(path np.Path) (*Dir, *np.Err) {
	d := &Dir{}
	o, err := makeObj(path)
	if err != nil {
		return nil, err
	}
	d.Obj = o
	return d, nil
}

func (d *Dir) uxReadDir(cursor int) ([]*np.Stat, *np.Err) {
	var sts []*np.Stat
	dirents, err := ioutil.ReadDir(d.Path())
	if err != nil {
		return nil, np.MkErrError(err)
	}
	for _, e := range dirents {
		st := &np.Stat{}
		st.Name = e.Name()
		if e.IsDir() {
			st.Mode = np.DMDIR
		} else {
			st.Mode = 0
		}
		st.Mode = st.Mode | np.Tperm(0777)
		sts = append(sts, st)
	}
	sort.SliceStable(sts, func(i, j int) bool {
		return sts[i].Name < sts[j].Name
	})
	db.DLPrintf("UXD", "%v: uxReadDir %v\n", d, len(sts)-cursor)
	if cursor > len(sts) {
		return nil, nil
	} else {
		return sts[cursor:], nil
	}
}

func (d *Dir) ReadDir(ctx fs.CtxI, cursor int, cnt np.Tsize, v np.TQversion) ([]*np.Stat, *np.Err) {
	db.DLPrintf("UXD", "%v: ReadDir %v %v %v\n", ctx, d, cursor, cnt)
	dirents, err := d.uxReadDir(cursor)
	if err != nil {
		return nil, err
	}
	return dirents, nil
}

// nothing to do for directiories until we page directories
func (d *Dir) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {
	return nil, nil
}

// nothing to do for directories until we page directories
func (d *Dir) Close(ctx fs.CtxI, mode np.Tmode) *np.Err {
	return nil
}

// XXX how to delete ephemeral files after crash
func (d *Dir) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	p := d.path.Append(name).String()
	db.DLPrintf("UXD", "%v: Create %v %v %v %v\n", ctx, d, name, p, perm)
	if perm.IsDir() {
		// XXX O_CREATE/O_EXCL
		error := os.Mkdir(p, os.FileMode(perm&0777))
		if error != nil {
			return nil, np.MkErrError(error)
		}
		d1, err := makeDir(append(d.path, name))
		if err != nil {
			return nil, err
		}
		return d1, nil
	} else {
		file, error := os.OpenFile(p, uxFlags(m)|os.O_CREATE|os.O_EXCL, os.FileMode(perm&0777))
		if error != nil {
			if errors.Is(error, os.ErrExist) {
				return nil, np.MkErr(np.TErrExists, name)
			} else {
				return nil, np.MkErrError(error)
			}
		}
		f, err := makeFile(append(d.path, name))
		if err != nil {
			return nil, err
		}
		f.file = file
		return f, nil
	}
}

func (d *Dir) namei(ctx fs.CtxI, p np.Path, inodes []fs.FsObj) ([]fs.FsObj, np.Path, *np.Err) {
	db.DLPrintf("UXD", "%v: namei %v %v\n", ctx, d, p)
	fi, error := os.Stat(d.path.Append(p[0]).String())
	if error != nil {
		return inodes, nil, np.MkErr(np.TErrNotfound, p[0])
	}
	if len(p) == 1 {
		if fi.IsDir() {
			d1, err := makeDir(append(d.path, p[0]))
			if err != nil {
				return inodes, d.path, err
			}
			return append(inodes, d1), nil, nil
		} else {
			f, err := makeFile(append(d.path, p[0]))
			if err != nil {
				return inodes, d.path, err
			}
			return append(inodes, f), nil, nil
		}
	} else {
		d1, err := makeDir(append(d.path, p[0]))
		if err != nil {
			return inodes, d.path, err
		}
		inodes = append(inodes, d1)
		return d1.namei(ctx, p[1:], inodes)
	}
}

func (d *Dir) Lookup(ctx fs.CtxI, p np.Path) ([]fs.FsObj, np.Path, *np.Err) {
	db.DLPrintf("UXD", "%v: Lookup %v %v\n", ctx, d, p)
	if len(p) == 0 {
		return nil, nil, nil
	}
	fi, error := os.Stat(d.path.String())
	if error != nil {
		return nil, nil, np.MkErrError(error)
	}
	if !fi.IsDir() {
		return nil, nil, np.MkErr(np.TErrNotDir, d.path)
	}
	return d.namei(ctx, p, nil)
}

func (d *Dir) WriteDir(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	return 0, np.MkErr(np.TErrNotSupported, nil)
}

func (d *Dir) Renameat(ctx fs.CtxI, from string, dd fs.Dir, to string) *np.Err {
	oldPath := d.Path() + "/" + from
	newPath := dd.(*Dir).Path() + "/" + to
	db.DLPrintf("UXD", "%v: Renameat d:%v from:%v to:%v\n", ctx, d, from, to)
	err := os.Rename(oldPath, newPath)
	if err != nil {
		return np.MkErrError(err)
	}
	return nil
}

func (d *Dir) Remove(ctx fs.CtxI, name string) *np.Err {
	db.DLPrintf("UXD", "%v: Remove %v %v\n", ctx, d, name)
	err := os.Remove(d.Path() + "/" + name)
	if err != nil {
		return np.MkErrError(err)
	}
	return nil
}

func (d *Dir) Rename(ctx fs.CtxI, from, to string) *np.Err {
	oldPath := d.Path() + "/" + from
	newPath := d.Path() + "/" + to
	db.DLPrintf("UXD", "%v: Rename d:%v from:%v to:%v\n", ctx, d, from, to)
	err := os.Rename(oldPath, newPath)
	if err != nil {
		return np.MkErrError(err)
	}
	return nil
}
