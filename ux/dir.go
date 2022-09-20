package fsux

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"syscall"

	db "sigmaos/debug"
	"sigmaos/fs"
	np "sigmaos/ninep"
	"sigmaos/sorteddir"
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
		return UxTo9PError(err, d.pathName.Base())
	}
	for _, e := range dirents {
		if st, err := ustat(d.pathName.Copy().Append(e.Name())); err != nil {
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
	d.sd = sorteddir.MkSortedDir()
	return nil
}

// XXX O_CREATE/O_EXCL
func (d *Dir) mkDir(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (*Dir, *np.Err) {
	p := d.pathName.Append(name).String()
	error := os.Mkdir(p, os.FileMode(perm&0777))
	if error != nil {
		return nil, UxTo9PError(error, name)
	}
	d1, err := makeDir(append(d.pathName, name))
	if err != nil {
		return nil, err
	}
	return d1, nil
}

func (d *Dir) mkFile(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	p := d.pathName.Append(name).String()
	fd, error := syscall.Open(p, uxFlags(m)|syscall.O_CREAT|syscall.O_EXCL, uint32(perm&0777))
	if error != nil {
		return nil, UxTo9PError(error, name)
	}
	f, err := makeFile(append(d.pathName, name))
	if err != nil {
		return nil, err
	}
	f.fd = fd
	return f, nil
}

func (d *Dir) mkPipe(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	p := d.pathName.Append(name).String()
	error := syscall.Mkfifo(p, uint32(perm&0777))
	if error != nil {
		return nil, UxTo9PError(error, name)
	}
	f, err := makePipe(ctx, append(d.pathName, name))
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (d *Dir) mkSym(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	p := d.pathName.Append(name)
	log.Printf("mkSym %s\n", p)
	s, err := makeSymlink(p, true)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// XXX how to delete ephemeral files after crash
func (d *Dir) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	db.DPrintf("UXD", "%v: Create %v n %v perm %v m %v\n", ctx, d, name, perm, m)
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

func (d *Dir) LookupPath(ctx fs.CtxI, path np.Path) ([]fs.FsObj, fs.FsObj, np.Path, *np.Err) {
	name := path[0]
	db.DPrintf("UXD", "%v: Lookup %v %v\n", ctx, d, name)
	st, err := ustat(d.pathName.Append(name))
	if err != nil {
		db.DPrintf("UXD", "%v: Lookup %v %v err %v\n", ctx, d, name, err)
		return nil, nil, path, err
	}
	db.DPrintf("UXD", "%v: Lookup %v %v st %v\n", ctx, d, name, st)
	var o fs.FsObj
	if st.Mode.IsDir() {
		o, err = makeDir(append(d.pathName, name))
		if err != nil {
			return nil, nil, path, err
		}
	} else if st.Mode.IsSymlink() {
		o, err = makeSymlink(append(d.pathName, name), false)
		if err != nil {
			return nil, nil, path, err
		}
	} else if st.Mode.IsPipe() {
		o, err = makePipe(ctx, append(d.pathName, name))
		if err != nil {
			return nil, nil, path, err
		}
	} else {
		o, err = makeFile(append(d.pathName, name))
		if err != nil {
			return nil, nil, path, err
		}
	}
	return []fs.FsObj{o}, o, path[1:], nil
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
		return UxTo9PError(err, to)
	}
	return nil
}

func (d *Dir) Remove(ctx fs.CtxI, name string) *np.Err {
	db.DPrintf("UXD", "%v: Remove %v %v\n", ctx, d, name)
	p := d.pathName.Copy().Append(name)
	o, err := makeObj(p)
	if err != nil {
		return err
	}
	error := os.Remove(p.String())
	if error != nil {
		return UxTo9PError(error, name)
	}
	if o.Perm().IsPipe() {
		pipe := fsux.ot.AllocRef(o.path, nil)
		if pipe != nil {
			pipe.(*Pipe).Unlink()
		}
	}
	return nil
}

func (d *Dir) Rename(ctx fs.CtxI, from, to string) *np.Err {
	oldPath := d.PathName() + "/" + from
	newPath := d.PathName() + "/" + to
	db.DPrintf("UXD", "%v: Rename d:%v from:%v to:%v\n", ctx, d, from, to)
	error := os.Rename(oldPath, newPath)
	if error != nil {
		return UxTo9PError(error, to)
	}
	// XXX unlink on pipe, if pipe
	return nil
}
