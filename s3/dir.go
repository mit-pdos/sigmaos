package fss3

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/umpc/go-sortedmap"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

type Dir struct {
	*Obj
	sync.Mutex
	dents *sortedmap.SortedMap
}

func (d *Dir) String() string {
	s := d.Obj.String()
	return s + fmt.Sprintf(" dents %v", d.dents)
}

func cmp(a, b interface{}) bool {
	if a == b {
		return true
	}
	return false
}

func makeDir(bucket string, key np.Path, perm np.Tperm) *Dir {
	o := makeObj(bucket, key, perm)
	dir := &Dir{}
	dir.Obj = o
	dir.dents = sortedmap.New(100, cmp)
	return dir
}

func (d *Dir) s3ReadDir(fss3 *Fss3) *np.Err {
	maxKeys := 0
	key := d.key.String()
	if key != "" {
		key = key + "/"
	}
	params := &s3.ListObjectsV2Input{
		Bucket:    &d.bucket,
		Prefix:    aws.String(key),
		Delimiter: aws.String("/"),
	}
	p := s3.NewListObjectsV2Paginator(fss3.client, params,
		func(o *s3.ListObjectsV2PaginatorOptions) {
			if v := int32(maxKeys); v != 0 {
				o.Limit = v
			}
		})
	for p.HasMorePages() {
		page, err := p.NextPage(context.TODO())
		if err != nil {
			return np.MkErr(np.TErrInval, err)
		}
		for _, obj := range page.Contents {
			db.DPrintf("FSS30", "key %v\n", *obj.Key)
			n := strings.TrimPrefix(*obj.Key, key)
			d.dents.Insert(n, np.Tperm(0777))
		}
		for _, obj := range page.CommonPrefixes {
			db.DPrintf("FSS30", "prefix %v\n", *obj.Prefix)
			n := strings.TrimPrefix(*obj.Prefix, key)
			d.dents.Insert(strings.TrimRight(n, "/"), np.DMDIR)
		}
	}
	d.sz = np.Tlength(d.dents.Len()) // makeup size
	d.init = true
	db.DPrintf("FSS3", "s3ReadDirL: dir %v\n", d)
	return nil
}

func (d *Dir) fill() *np.Err {
	if !d.init {
		if err := d.s3ReadDir(fss3); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dir) dirents() []fs.FsObj {
	d.Lock()
	defer d.Unlock()
	dents := make([]fs.FsObj, 0, d.dents.Len())
	d.dents.IterFunc(false, func(rec sortedmap.Record) bool {
		dents = append(dents, makeFsObj(d.bucket, rec.Val.(np.Tperm), d.key.Append(rec.Key.(string))))
		return true
	})
	return dents
}

func (d *Dir) Stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
	d.Lock()
	defer d.Unlock()
	db.DPrintf("FSS3", "Stat dir %v\n", d)
	if err := d.fill(); err != nil {
		return nil, err
	}
	return d.stat(ctx)
}

func (d *Dir) lookupDirent(name string) fs.FsObj {
	d.Lock()
	defer d.Unlock()

	if p, ok := d.dents.Get(name); ok {
		return makeFsObj(d.bucket, p.(np.Tperm), d.key.Append(name))
	}
	return nil
}

func (d *Dir) insertDirent(name string, perm np.Tperm) fs.FsObj {
	d.Lock()
	defer d.Unlock()
	if _, ok := d.dents.Get(name); ok {
		return nil
	}
	d.dents.Insert(name, perm)
	return makeFsObj(d.bucket, perm, d.key.Append(name))
}

func (d *Dir) delDirent(name string) {
	d.Lock()
	defer d.Unlock()
	d.dents.Delete(name)
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
		return nil, nil, nil, err
	}
	o := d.lookupDirent(p[0])
	if o == nil {
		db.DPrintf("FSS3", "%v: namei %v not found\n", d, p[0])
		return qids, d, p, np.MkErr(np.TErrNotfound, p[0])
	}
	qids = append(qids, o.Qid())
	if len(p) == 1 {
		db.DPrintf("FSS3", "%v: namei final %v %v\n", ctx, qids, o)
		return qids, o, nil, nil
	} else {
		return o.(*Dir).namei(ctx, p[1:], qids)
	}
}

func (d *Dir) Lookup(ctx fs.CtxI, p np.Path) ([]np.Tqid, fs.FsObj, np.Path, *np.Err) {
	db.DPrintf("FSS3", "%v: Lookup %v '%v'", ctx, d, p)
	if len(p) == 0 {
		return nil, nil, nil, nil
	}
	if !d.Perm().IsDir() {
		return nil, nil, nil, np.MkErr(np.TErrNotDir, d)
	}
	qids, o, err := nameiObj(ctx, d.bucket, p)
	if err == nil {
		db.DPrintf("FSS3", "%v: nameiObj %v %v\n", ctx, qids, o)
		return qids, o, nil, nil
	}
	// maybe path names a directory
	return d.namei(ctx, p, nil)
}

func (d *Dir) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {
	db.DPrintf("FSS3", "open dir %v (%T) %v\n", d, d, m)
	if err := d.fill(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Dir) ReadDir(ctx fs.CtxI, cursor int, cnt np.Tsize, v np.TQversion) ([]*np.Stat, *np.Err) {
	var dirents []*np.Stat
	db.DPrintf("FSS3", "ReadDir %v\n", d)
	if err := d.fill(); err != nil {
		return nil, err
	}
	for _, o1 := range d.dirents() {
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
	db.DPrintf("FSS3", "Create %v name: %v\n", d, name)
	if err := d.fill(); err != nil {
		return nil, err
	}
	o := d.insertDirent(name, perm)
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
		Bucket: &d.bucket,
		Key:    &key,
	}
	db.DPrintf("FSS3", "Delete %v key %v name %v\n", d, key, name)
	_, err := fss3.client.DeleteObject(context.TODO(), input)
	if err != nil {
		return np.MkErrError(err)
	}
	d.delDirent(name)
	return nil
}

func (d *Dir) Rename(ctx fs.CtxI, from, to string) *np.Err {
	return np.MkErr(np.TErrNotSupported, "Rename")
}

// ===== The following functions are needed to make an s3 dir of type a dir.DirImpl

func (d *Dir) SetMtime(mtime int64) {
	db.DFatalf("Unimplemented")
}

func (d *Dir) Mtime() int64 {
	db.DFatalf("Unimplemented")
	return 0
}

func (d *Dir) SetParent(di fs.Dir) {
	db.DFatalf("Unimplemented")
}

func (d *Dir) Snapshot(fs.SnapshotF) []byte {
	db.DFatalf("Unimplemented")
	return nil
}

func (d *Dir) Unlink() {
	db.DFatalf("Unimplemented")
}

func (d *Dir) VersionInc() {
	db.DFatalf("Unimplemented")
}
