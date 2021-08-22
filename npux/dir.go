package npux

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

func (npux *NpUx) makeDir(path []string, t np.Tperm, p *Dir) *Dir {
	d := &Dir{}
	npux.mu.Lock()
	defer npux.mu.Unlock()
	d.Obj = npux.makeObjL(path, t, p)
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
func (d *Dir) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.NpObj, error) {
	p := np.Join(append(d.path, name))
	db.DLPrintf("UXD", "%v: Create %v %v %v %v\n", ctx, d, name, p, perm)
	var err error
	if perm.IsDir() {
		err = os.Mkdir(p, os.FileMode(perm&0777))
		if err != nil {
			return nil, err
		}
		d1 := d.npux.makeDir(append(d.path, name), 0, d)
		return d1, nil
	} else {
		file, err := os.OpenFile(p, uxFlags(m)|os.O_CREATE, os.FileMode(perm&0777))
		if err != nil {
			return nil, err
		}
		f := d.npux.makeFile(append(d.path, name), 0, d)
		if file != nil {
			f.file = file
		}
		return f, nil
	}
}

// XXX intermediate dirs?
func (d *Dir) Lookup(ctx fs.CtxI, p []string) ([]fs.NpObj, []string, error) {
	db.DLPrintf("UXD", "%v: Lookup %v %v\n", ctx, d, p)
	fi, err := os.Stat(np.Join(append(d.path, p...)))
	if err != nil {
		return nil, nil, err
	}
	if fi.IsDir() {
		d := d.npux.makeDir(append(d.path, p...), np.DMDIR, d)
		return []fs.NpObj{d}, nil, nil
	} else {
		f := d.npux.makeFile(append(d.path, p...), np.Tperm(0), d)
		return []fs.NpObj{f}, nil, nil
	}
}

func (d *Dir) WriteDir(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	return 0, fmt.Errorf("not supported")
}

func (d *Dir) Renameat(ctx fs.CtxI, from string, od fs.NpObjDir, to string) error {
	return fmt.Errorf("not supported")
}
