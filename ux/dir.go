package fsux

import (
	"fmt"
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

func (d *Dir) String() string {
	return fmt.Sprintf("o %v sd %v", d.Obj, d.sd)
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
	dirents, err := ioutil.ReadDir(d.PathName())
	if err != nil {
		return UxTo9PError(err)
	}
	for _, e := range dirents {
		if st, err := ustat(d.path.Copy().Append(e.Name())); err != nil {
			// another proc may have removed the file
			continue
		} else {
			d.sd.Insert(st.Name, st)
		}
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

func (d *Dir) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {
	if err := d.uxReadDir(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (d *Dir) Close(ctx fs.CtxI, mode np.Tmode) *np.Err {
	return nil
}

// XXX O_CREATE/O_EXCL
func (d *Dir) mkDir(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (*Dir, *np.Err) {
	p := d.path.Append(name).String()
	error := os.Mkdir(p, os.FileMode(perm&0777))
	if error != nil {
		return nil, UxTo9PError(error)
	}
	d1, err := makeDir(append(d.path, name))
	if err != nil {
		return nil, err
	}
	return d1, nil
}

func (d *Dir) mkFile(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	p := d.path.Append(name).String()
	file, error := os.OpenFile(p, uxFlags(m)|os.O_CREATE|os.O_EXCL, os.FileMode(perm&0777))
	if error != nil {
		return nil, UxTo9PError(error)
	}
	f, err := makeFile(append(d.path, name))
	if err != nil {
		return nil, err
	}
	f.file = file
	return f, nil
}

func (d *Dir) mkPipe(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	return nil, np.MkErr(np.TErrNotSupported, "pipe")
}

func (d *Dir) mkSym(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	return nil, np.MkErr(np.TErrNotSupported, "sym")
}

// XXX how to delete ephemeral files after crash
func (d *Dir) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	db.DPrintf("UXD", "%v: Create %v %v %v %v\n", ctx, d, name, perm)
	if perm.IsDir() {
		return d.mkDir(ctx, name, perm, m)
	} else if perm.IsPipe() {
		return d.mkPipe(ctx, name, perm, m)
	} else if perm.IsSymlink() {
		return d.mkSym(ctx, name, perm, m)
	} else {
		return d.mkFile(ctx, name, perm, m)
	}
}

func (d *Dir) namei(ctx fs.CtxI, p np.Path, objs []fs.FsObj) ([]fs.FsObj, fs.FsObj, np.Path, *np.Err) {
	db.DPrintf("UXD", "%v: namei %v %v\n", ctx, d, p)
	fi, error := os.Stat(d.path.Append(p[0]).String())
	if error != nil {
		return objs, d, d.path, np.MkErr(np.TErrNotfound, p[0])
	}
	if len(p) == 1 {
		if fi.IsDir() {
			d1, err := makeDir(append(d.path, p[0]))
			if err != nil {
				return objs, d1, d.path, err
			}
			return append(objs, d1), d1, nil, nil
		} else {
			f, err := makeFile(append(d.path, p[0]))
			if err != nil {
				return objs, f, d.path, err
			}
			return append(objs, f), f, nil, nil
		}
	} else {
		d1, err := makeDir(append(d.path, p[0]))
		if err != nil {
			return objs, d, d.path, err
		}
		objs = append(objs, d1)
		return d1.namei(ctx, p[1:], objs)
	}
}

func (d *Dir) Lookup(ctx fs.CtxI, p np.Path) ([]fs.FsObj, fs.FsObj, np.Path, *np.Err) {
	db.DPrintf("UXD", "%v: Lookup %v %v\n", ctx, d, p)
	if len(p) == 0 {
		return nil, nil, nil, nil
	}
	fi, error := os.Stat(d.path.String())
	if error != nil {
		return nil, nil, nil, UxTo9PError(error)
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
	oldPath := d.PathName() + "/" + from
	newPath := dd.(*Dir).PathName() + "/" + to
	db.DPrintf("UXD", "%v: Renameat d:%v from:%v to:%v\n", ctx, d, from, to)
	err := os.Rename(oldPath, newPath)
	if err != nil {
		return UxTo9PError(err)
	}
	return nil
}

func (d *Dir) Remove(ctx fs.CtxI, name string) *np.Err {
	db.DPrintf("UXD", "%v: Remove %v %v\n", ctx, d, name)
	p := d.path.Copy().Append(name)
	error := os.Remove(p.String())
	if error != nil {
		return UxTo9PError(error)
	}
	return nil
}

func (d *Dir) Rename(ctx fs.CtxI, from, to string) *np.Err {
	oldPath := d.PathName() + "/" + from
	newPath := d.PathName() + "/" + to
	db.DPrintf("UXD", "%v: Rename d:%v from:%v to:%v\n", ctx, d, from, to)
	error := os.Rename(oldPath, newPath)
	if error != nil {
		return UxTo9PError(error)
	}
	return nil
}
