package fsux

import (
	"fmt"
	"io/ioutil"
	"os"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
)

type Dir struct {
	*Obj
}

func (fsux *FsUx) makeDir(path []string, t np.Tperm, p *Dir) *Dir {
	d := &Dir{}
	fsux.mu.Lock()
	defer fsux.mu.Unlock()
	d.Obj = fsux.makeObjL(path, t, p)
	return d
}

func (d *Dir) uxReadDir() ([]*np.Stat, error) {
	var sts []*np.Stat
	dirents, err := ioutil.ReadDir(d.Path())
	if err != nil {
		return nil, err
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
	db.DLPrintf("UXD", "%v: uxReadDir %v\n", d, sts)
	return sts, nil
}

func (d *Dir) ReadDir(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]*np.Stat, error) {
	db.DLPrintf("UXD", "%v: ReadDir %v %v %v\n", ctx, d, off, cnt)
	dirents, err := d.uxReadDir()
	if err != nil {
		return nil, err
	}
	return dirents, nil
}

// XXX close
func (d *Dir) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, error) {
	p := np.Join(append(d.path, name))
	db.DLPrintf("UXD", "%v: Create %v %v %v %v\n", ctx, d, name, p, perm)
	var err error
	if perm.IsDir() {
		err = os.Mkdir(p, os.FileMode(perm&0777))
		if err != nil {
			return nil, err
		}
		d1 := d.fsux.makeDir(append(d.path, name), 0, d)
		return d1, nil
	} else {
		file, err := os.OpenFile(p, uxFlags(m)|os.O_CREATE, os.FileMode(perm&0777))
		if err != nil {
			return nil, err
		}
		f := d.fsux.makeFile(append(d.path, name), 0, d)
		if file != nil {
			f.file = file
		}
		return f, nil
	}
}

func (d *Dir) Lookup(ctx fs.CtxI, p []string) ([]fs.FsObj, []string, error) {
	db.DLPrintf("UXD", "%v: Lookup %v %v\n", ctx, d, p)
	fi, err := os.Stat(np.Join(d.path))
	if err != nil {
		return nil, nil, err
	}
	if !fi.IsDir() {
		return nil, nil, fmt.Errorf("Not a directory")
	}
	fi, err = os.Stat(np.Join(append(d.path, p[0])))
	if err != nil {
		return nil, nil, fmt.Errorf("file not found")
	}
	if len(p) == 1 {
		if fi.IsDir() {
			d1 := d.fsux.makeDir(append(d.path, p[0]), np.DMDIR, d)
			return []fs.FsObj{d1}, nil, nil
		} else {
			f := d.fsux.makeFile(append(d.path, p[0]), np.Tperm(0), d)
			return []fs.FsObj{f}, nil, nil
		}
	} else {
		d1 := d.fsux.makeDir(append(d.path, p[0]), np.DMDIR, d)
		return d1.Lookup(ctx, p[1:])
	}
}

func (d *Dir) WriteDir(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	return 0, fmt.Errorf("not supported")
}

func (d *Dir) Renameat(ctx fs.CtxI, from string, od fs.Dir, to string) error {
	return fmt.Errorf("not supported")
}

func (d *Dir) Remove(ctx fs.CtxI, name string) error {
	db.DLPrintf("UXD", "%v: Remove %v %v\n", ctx, d, name)
	err := os.Remove(d.Path() + "/" + name)
	return err
}

func (d *Dir) Rename(ctx fs.CtxI, from, to string) error {
	oldPath := d.Path() + "/" + from
	newPath := d.Path() + "/" + to
	db.DLPrintf("UXD", "%v: Rename d:%v from:%v to:%v\n", ctx, d, from, to)
	err := os.Rename(oldPath, newPath)
	if err != nil {
		return err
	}
	return nil
}
