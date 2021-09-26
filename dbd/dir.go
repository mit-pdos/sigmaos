package dbd

import (
	"fmt"
	"log"

	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
)

// XXX locking
type Dir struct {
	*inode.Inode
	dirents map[string]fs.FsObj
}

func makeDir(db *Database, path []string, perm np.Tperm, p *Dir) *Dir {
	d := &Dir{}
	d.Inode = inode.MakeInode("", perm, p)
	d.dirents = make(map[string]fs.FsObj)
	return d
}

func makeRoot(db *Database) *Dir {
	d := makeDir(db, []string{""}, np.DMDIR, nil)
	c := &Clone{}
	c.Inode = inode.MakeInode("", 0, d)
	d.dirents["clone"] = c
	return d
}

func (d *Dir) create(name string, o fs.FsObj) {
	d.dirents[name] = o
}

func (d *Dir) readDir(ctx fs.CtxI) ([]*np.Stat, error) {
	var sts []*np.Stat
	for _, o := range d.dirents {
		if st, err := o.Stat(ctx); err == nil {
			sts = append(sts, st)
		}
	}
	log.Printf("readdir %v\n", sts)
	return sts, nil
}

func (d *Dir) ReadDir(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]*np.Stat, error) {
	log.Printf("readdir %v\n", d)
	dirents, err := d.readDir(ctx)
	if err != nil {
		return nil, err
	}
	return dirents, nil
}

func (d *Dir) namei(ctx fs.CtxI, path []string, objs []fs.FsObj) ([]fs.FsObj, []string, error) {
	if e, ok := d.dirents[path[0]]; ok {
		objs = append(objs, e)
		switch i := e.(type) {
		case *Dir:
			if len(path) == 1 { // done?
				return objs, nil, nil
			}
			return i.namei(ctx, path[1:], objs)
		default:
			return objs, path[1:], nil
		}
	} else {
		return nil, nil, fmt.Errorf("file not found")
	}
}

func (d *Dir) Lookup(ctx fs.CtxI, p []string) ([]fs.FsObj, []string, error) {
	log.Printf("path %v\n", p)
	if len(p) == 0 {
		return nil, nil, nil
	}
	objs := []fs.FsObj{}
	objs, rest, err := d.namei(ctx, p, objs)
	if err == nil {
		return objs, rest, err
	} else {
		return nil, rest, err // XXX was nil?
	}
}

func (d *Dir) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, error) {
	return nil, fmt.Errorf("not supported")
}

func (d *Dir) WriteDir(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	return 0, fmt.Errorf("not supported")
}

func (d *Dir) Renameat(ctx fs.CtxI, from string, od fs.Dir, to string) error {
	return fmt.Errorf("not supported")
}

func (d *Dir) Remove(fs.CtxI, string) error {
	return fmt.Errorf("not supported")
}

func (d *Dir) Rename(fs.CtxI, string, string) error {
	return fmt.Errorf("not supported")
}
