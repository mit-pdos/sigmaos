package fsux

import (
	"errors"
	"io/ioutil"
	"os"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/sorteddir"
)

type Dir struct {
	*Obj
	sd *sorteddir.SortedDir
}

func makeDir(path np.Path) (*Dir, *np.Err) {
	d := &Dir{}
	o, err := makeObj(path)
	if err != nil {
		return nil, err
	}
	d.Obj = o
	d.sd = sorteddir.MkSortedDir()
	return d, nil
}

func (d *Dir) uxReadDir() *np.Err {
	dirents, err := ioutil.ReadDir(d.Path())
	if err != nil {
		return np.MkErrError(err)
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
		fi, err := os.Stat(d.path.String() + "/" + st.Name)
		if err != nil {
			// another proc may have removed the file
			continue
		}
		st.Length = np.Tlength(fi.Size())
		d.sd.Insert(st.Name, st)
	}
	db.DPrintf("UXD", "%v: uxReadDir %v\n", d, d.sd.Len())
	return nil
}

func (d *Dir) ReadDir(ctx fs.CtxI, cursor int, cnt np.Tsize, v np.TQversion) ([]*np.Stat, *np.Err) {
	db.DPrintf("UXD", "%v: ReadDir %v %v %v\n", ctx, d, cursor, cnt)
	dents := make([]*np.Stat, 0, d.sd.Len())
	d.sd.Iter(func(n string, e interface{}) bool {
		dents = append(dents, e.(*np.Stat))
		return true
	})
	if cursor > len(dents) {
		return nil, nil
	} else {
		return dents[cursor:], nil
	}
}

// nothing to do for directiories until we page directories
func (d *Dir) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {
	if err := d.uxReadDir(); err != nil {
		return nil, err
	}
	return nil, nil
}

// nothing to do for directories until we page directories
func (d *Dir) Close(ctx fs.CtxI, mode np.Tmode) *np.Err {
	return nil
}

// XXX how to delete ephemeral files after crash
func (d *Dir) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	p := d.path.Append(name).String()
	db.DPrintf("UXD", "%v: Create %v %v %v %v\n", ctx, d, name, p, perm)
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

func (d *Dir) namei(ctx fs.CtxI, p np.Path, qids []np.Tqid) ([]np.Tqid, fs.FsObj, np.Path, *np.Err) {
	db.DPrintf("UXD", "%v: namei %v %v\n", ctx, d, p)
	fi, error := os.Stat(d.path.Append(p[0]).String())
	if error != nil {
		return qids, d, d.path, np.MkErr(np.TErrNotfound, p[0])
	}
	if len(p) == 1 {
		if fi.IsDir() {
			d1, err := makeDir(append(d.path, p[0]))
			if err != nil {
				return qids, d1, d.path, err
			}
			return append(qids, d1.Qid()), d1, nil, nil
		} else {
			f, err := makeFile(append(d.path, p[0]))
			if err != nil {
				return qids, f, d.path, err
			}
			return append(qids, f.Qid()), f, nil, nil
		}
	} else {
		d1, err := makeDir(append(d.path, p[0]))
		if err != nil {
			return qids, d, d.path, err
		}
		qids = append(qids, d1.Qid())
		return d1.namei(ctx, p[1:], qids)
	}
}

func (d *Dir) Lookup(ctx fs.CtxI, p np.Path) ([]np.Tqid, fs.FsObj, np.Path, *np.Err) {
	db.DPrintf("UXD", "%v: Lookup %v %v\n", ctx, d, p)
	if len(p) == 0 {
		return nil, nil, nil, nil
	}
	fi, error := os.Stat(d.path.String())
	if error != nil {
		return nil, nil, nil, np.MkErrError(error)
	}
	if !fi.IsDir() {
		return nil, nil, nil, np.MkErr(np.TErrNotDir, d.path)
	}
	return d.namei(ctx, p, nil)
}

func (d *Dir) WriteDir(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	return 0, np.MkErr(np.TErrNotSupported, nil)
}

func (d *Dir) Renameat(ctx fs.CtxI, from string, dd fs.Dir, to string) *np.Err {
	oldPath := d.Path() + "/" + from
	newPath := dd.(*Dir).Path() + "/" + to
	db.DPrintf("UXD", "%v: Renameat d:%v from:%v to:%v\n", ctx, d, from, to)
	err := os.Rename(oldPath, newPath)
	if err != nil {
		return np.MkErrError(err)
	}
	return nil
}

func (d *Dir) Remove(ctx fs.CtxI, name string) *np.Err {
	db.DPrintf("UXD", "%v: Remove %v %v\n", ctx, d, name)
	err := os.Remove(d.Path() + "/" + name)
	if err != nil {
		return np.MkErrError(err)
	}
	return nil
}

func (d *Dir) Rename(ctx fs.CtxI, from, to string) *np.Err {
	oldPath := d.Path() + "/" + from
	newPath := d.Path() + "/" + to
	db.DPrintf("UXD", "%v: Rename d:%v from:%v to:%v\n", ctx, d, from, to)
	err := os.Rename(oldPath, newPath)
	if err != nil {
		return np.MkErrError(err)
	}
	return nil
}
