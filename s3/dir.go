package fss3

import (
	"context"
	"sort"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

type Dir struct {
	*Obj
}

func makeDir(key np.Path, perm np.Tperm) *Dir {
	o := makeObj(key, perm)
	dir := &Dir{}
	dir.Obj = o
	return dir
}

func (d *Dir) Stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
	db.DPrintf("FSS3", "Stat Dir: %v\n", d)
	if err := d.fill(); err != nil {
		return nil, err
	}
	return d.info.stat(), nil
}

// fake a stat without filling
func (d *Dir) stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
	db.DPrintf("FSS3", "stat Dir: %v\n", d)
	st := &np.Stat{}
	st.Name = d.key.Base()
	st.Mode = d.perm | np.Tperm(0777)
	st.Qid = qid(d.perm, d.key)
	return st, nil
}

func (d *Dir) namei(ctx fs.CtxI, p np.Path, qids []np.Tqid) ([]np.Tqid, fs.FsObj, np.Path, *np.Err) {
	db.DPrintf("FSS3", "%v: namei %v\n", d, p)
	if err := d.fill(); err != nil {
		db.DPrintf("FSS3", "%v: fill err %v\n", d, err)
		return nil, nil, nil, err
	}
	o := d.info.lookupDirent(p[0])
	if o == nil {
		db.DPrintf("FSS3", "%v: namei %v not found\n", d, p[0])
		return qids, d, p, np.MkErr(np.TErrNotfound, p[0])
	}
	qids = append(qids, o.Qid())
	if len(p) == 1 {
		db.DPrintf("FSS3", "%v: namei %v %v\n", ctx, qids, o)
		return qids, o, nil, nil
	} else {
		return o.(*Dir).namei(ctx, p[1:], qids)
	}
}

func (d *Dir) Lookup(ctx fs.CtxI, p np.Path) ([]np.Tqid, fs.FsObj, np.Path, *np.Err) {
	db.DPrintf("FSS3", "%v: Lookup %v '%v'\n", ctx, d, p)
	if len(p) == 0 {
		return nil, nil, nil, nil
	}
	if !d.Perm().IsDir() {
		return nil, nil, nil, np.MkErr(np.TErrNotDir, d)
	}
	qids, o, err := nameiObj(ctx, p)
	if err == nil {
		db.DPrintf("FSS3", "%v: nameiObj %v %v\n", ctx, qids, o)
		return qids, o, nil, nil
	}
	// maybe path names a directory
	return d.namei(ctx, p, nil)
}

func (d *Dir) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {
	db.DPrintf("FSS3", "open %v (%T) %v\n", d, d, m)
	if err := d.fill(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Dir) ReadDir(ctx fs.CtxI, cursor int, cnt np.Tsize, v np.TQversion) ([]*np.Stat, *np.Err) {
	var dirents []*np.Stat
	db.DPrintf("FSS3", "readDir: %v\n", d)
	if err := d.fill(); err != nil {
		return nil, err
	}
	for _, o1 := range d.info.dirents() {
		var st *np.Stat
		var err *np.Err
		switch v := o1.(type) {
		case *Dir:
			st, err = v.stat(ctx)
		case *Obj:
			st, err = v.Stat(ctx)
		}
		if err != nil {
			return nil, err
		}
		dirents = append(dirents, st)
	}
	sort.SliceStable(dirents, func(i, j int) bool {
		return dirents[i].Name < dirents[j].Name
	})
	d.sz = npcodec.MarshalSizeDir(dirents)
	if cursor > len(dirents) {
		return nil, nil
	} else {
		return dirents[cursor:], nil
	}
}

func (d *Dir) WriteDir(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	return 0, np.MkErr(np.TErrIsdir, d)
	// return np.Tsize(len(b)), nil
}

func (d *Dir) CreateDir(ctx fs.CtxI, name string, perm np.Tperm) (fs.FsObj, *np.Err) {
	// create a fake "file" in "dir" to materialize it
	//if _, err := dir.Create(ctx, "_._", perm&0777, m); err != nil {
	//	db.DPrintf("FSS3", "Create x err %v\n", err)
	//	return nil, err
	//}
	return nil, nil
}

func (d *Dir) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	if err := d.fill(); err != nil {
		return nil, err
	}
	db.DPrintf("FSS3", "Create %v name: %v\n", d, name)
	o := d.info.insertDirent(name, perm)
	if o == nil {
		return nil, np.MkErr(np.TErrExists, name)
	}
	if perm.IsFile() && m == np.OWRITE {
		o.(*Obj).setupWriter()
	}
	return o, nil
}

func (d *Dir) Renameat(ctx fs.CtxI, from string, od fs.Dir, to string) *np.Err {
	return np.MkErr(np.TErrNotSupported, "Renameat")
}

func (d *Dir) Remove(ctx fs.CtxI, name string) *np.Err {
	if err := d.fill(); err != nil {
		return err
	}
	key := d.key.Append(name).String()
	input := &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	db.DPrintf("FSS3", "Delete %v key %v name %v\n", d, key, name)
	_, err := fss3.client.DeleteObject(context.TODO(), input)
	if err != nil {
		return np.MkErrError(err)
	}
	d.info.delDirent(name)
	return nil
}

func (d *Dir) Rename(ctx fs.CtxI, from, to string) *np.Err {
	return np.MkErr(np.TErrNotSupported, "Rename")
}
