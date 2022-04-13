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

func (d *Dir) fillDir() *np.Err {
	if d.info == nil {
		i := cache.lookup(d.key)
		if i != nil {
			d.info = i
			return nil
		}
		i, err := s3ReadDirL(fss3, d.key)
		if err != nil {
			return err
		}
		d.info = i
		return nil
	}
	return nil
}

func (d *Dir) Qid() np.Tqid {
	d.fillDir()
	return np.MakeQid(np.Qtype(d.Perm()>>np.QTYPESHIFT),
		np.TQversion(0), np.Tpath(0)) // o.ino
}

func (d *Dir) Stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
	db.DPrintf("FSS3", "Stat Dir: %v\n", d)
	if err := d.fillDir(); err != nil {
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
	if err := d.fillDir(); err != nil {
		return nil, nil, nil, err
	}
	o1 := d.info.lookupDirent(p[0])
	if o1 == nil {
		return qids, d, nil, np.MkErr(np.TErrNotfound, p[0])
	}
	qids = append(qids, o1.Qid())
	if len(p) == 1 {
		return qids, o1, nil, nil
	} else {
		return o1.(*Dir).namei(ctx, p[1:], qids)
	}
}

func (d *Dir) Lookup(ctx fs.CtxI, p np.Path) ([]np.Tqid, fs.FsObj, np.Path, *np.Err) {
	db.DPrintf("FSS3", "%v: lookup %v %v\n", ctx, d, p)
	if len(p) == 0 {
		return nil, nil, nil, nil
	}
	if !d.Perm().IsDir() {
		return nil, nil, nil, np.MkErr(np.TErrNotDir, d)
	}
	return d.namei(ctx, p, nil)
}

func (d *Dir) ReadDir(ctx fs.CtxI, cursor int, cnt np.Tsize, v np.TQversion) ([]*np.Stat, *np.Err) {
	var dirents []*np.Stat
	db.DPrintf("FSS3", "readDir: %v\n", d)
	if err := d.fillDir(); err != nil {
		return nil, err
	}
	for _, o1 := range d.info.dirEnts() {
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

func (d *Dir) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	if d.info == nil {
		if i, err := s3ReadDirL(fss3, d.key); err != nil {
			return nil, err
		} else {
			d.info = i
		}
	}
	if perm.IsDir() {
		// dir := makeDir(append(d.key, name))
		// create a fake "file" in "dir" to materialize it
		//if _, err := dir.Create(ctx, "_._", perm&0777, m); err != nil {
		//	db.DPrintf("FSS3", "Create x err %v\n", err)
		//	return nil, err
		//}
		o := d.info.insertDirent(name, np.DMDIR)
		return o, nil
	}
	key := d.key.Append(name).String()
	db.DPrintf("FSS3", "Create key: %v\n", key)
	input := &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	_, err := fss3.client.PutObject(context.TODO(), input)
	if err != nil {
		return nil, np.MkErrError(err)
	}
	// XXX ignored perm, only files not directories
	o := d.info.insertDirent(name, 0)
	if o == nil {
		return nil, np.MkErr(np.TErrExists, name)
	}
	if m == np.OWRITE {
		o.(*Obj).setupWriter()
	}
	return o, nil
}

func (d *Dir) Renameat(ctx fs.CtxI, from string, od fs.Dir, to string) *np.Err {
	return np.MkErr(np.TErrNotSupported, "Renameat")
}

func (d *Dir) Remove(ctx fs.CtxI, name string) *np.Err {
	key := d.key.Append(name).String()
	input := &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	db.DPrintf("FSS3", "Delete key: %v\n", key)
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
