package fss3

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/sorteddir"
)

type Dir struct {
	*Obj
	sync.Mutex
	dents *sorteddir.SortedDir
	sts   []*np.Stat
}

func (d *Dir) String() string {
	s := d.Obj.String()
	return s + fmt.Sprintf(" dents %v", d.dents)
}

func makeDir(bucket string, key np.Path, perm np.Tperm) *Dir {
	o := makeObj(bucket, key, perm)
	dir := &Dir{}
	dir.Obj = o
	dir.dents = sorteddir.MkSortedDir()
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
			if n != "_._" {
				d.dents.Insert(n, np.Tperm(0777))
			}
		}
		for _, obj := range page.CommonPrefixes {
			db.DPrintf("FSS30", "prefix %v\n", *obj.Prefix)
			n := strings.TrimPrefix(*obj.Prefix, key)
			d.dents.Insert(strings.TrimRight(n, "/"), np.DMDIR)
		}
	}
	d.sz = np.Tlength(d.dents.Len()) // makeup size
	db.DPrintf("FSS3", "s3ReadDirL: dir %v\n", d)
	return nil
}

func (d *Dir) fill() *np.Err {
	if err := d.s3ReadDir(fss3); err != nil {
		return err
	}
	return nil
}

func (d *Dir) dirents() []*Obj {
	d.Lock()
	defer d.Unlock()
	dents := make([]*Obj, 0, d.dents.Len())
	d.dents.Iter(func(n string, e interface{}) bool {
		dents = append(dents, makeObj(d.bucket, d.key.Append(n), e.(np.Tperm)))
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
	st := d.stat()
	st.Length = d.sz
	return st, nil
}

func (d *Dir) namei(ctx fs.CtxI, p np.Path, qids []np.Tqid) ([]np.Tqid, fs.FsObj, np.Path, *np.Err) {
	db.DPrintf("FSS3", "%v: namei %v\n", d, p)
	if err := d.fill(); err != nil {
		return nil, nil, nil, err
	}
	e, ok := d.dents.Lookup(p[0])
	if !ok {
		db.DPrintf("FSS3", "%v: namei %v not found\n", d, p[0])
		return qids, d, p, np.MkErr(np.TErrNotfound, p[0])
	}
	if len(p) == 1 {
		perm := e.(np.Tperm)
		var o fs.FsObj
		if perm.IsDir() {
			o = makeDir(d.bucket, d.key.Append(p[0]), perm)
		} else {
			o = makeObj(d.bucket, d.key.Append(p[0]), perm)
		}
		qids = append(qids, o.Qid())
		db.DPrintf("FSS3", "%v: namei final %v %v\n", ctx, qids, o)
		return qids, o, nil, nil
	} else {
		d := makeDir(d.bucket, d.key.Append(p[0]), e.(np.Tperm))
		qids = append(qids, d.Qid())
		return d.namei(ctx, p[1:], qids)
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
	// maybe p names a directory
	return d.namei(ctx, p, nil)
}

func (d *Dir) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {
	db.DPrintf("FSS3", "open dir %v (%T) %v\n", d, d, m)
	if err := d.fill(); err != nil {
		return nil, err
	}
	d.sts = make([]*np.Stat, 0, d.dents.Len())
	for _, o := range d.dirents() {
		d.sts = append(d.sts, o.stat())
	}
	d.sz = npcodec.MarshalSizeDir(d.sts)
	return d, nil
}

func (d *Dir) ReadDir(ctx fs.CtxI, cursor int, cnt np.Tsize, v np.TQversion) ([]*np.Stat, *np.Err) {
	db.DPrintf("FSS3", "ReadDir %v\n", d)

	if cursor > len(d.sts) {
		return nil, nil
	} else {
		return d.sts[cursor:], nil
	}
}

func (d *Dir) WriteDir(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	return 0, np.MkErr(np.TErrIsdir, d)
}

// Create a fake file in dir to materialize dir
func (d *Dir) CreateDir(ctx fs.CtxI, name string, perm np.Tperm) (fs.FsObj, *np.Err) {
	key := d.key.Append(name).Append("_._").String()
	db.DPrintf("FSS3", "CreateDir: %v\n", key)
	input := &s3.PutObjectInput{
		Bucket: &d.bucket,
		Key:    &key,
	}
	_, err := fss3.client.PutObject(context.TODO(), input)
	if err != nil {
		return nil, np.MkErrError(err)
	}
	o := makeFsObj(d.bucket, perm, d.key.Append(name))
	return o, nil
}

// XXX check that name != "_._"
func (d *Dir) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	db.DPrintf("FSS3", "Create %v name: %v\n", d, name)
	if err := d.fill(); err != nil {
		return nil, err
	}
	if perm.IsDir() {
		return d.CreateDir(ctx, name, perm)
	}
	o := makeFsObj(d.bucket, perm, d.key.Append(name))
	if perm.IsFile() && m == np.OWRITE {
		o.(*Obj).setupWriter()
	}
	return o, nil
}

func (d *Dir) Renameat(ctx fs.CtxI, from string, od fs.Dir, to string) *np.Err {
	return np.MkErr(np.TErrNotSupported, "Renameat")
}

// XXX check in case of directory that it is empty
func (d *Dir) Remove(ctx fs.CtxI, name string) *np.Err {
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
