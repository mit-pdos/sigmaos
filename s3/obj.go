package fss3

import (
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "sigmaos/debug"
	"sigmaos/sessp"
    "sigmaos/serr"
	"sigmaos/fs"
	"sigmaos/path"
	sp "sigmaos/sigmap"
)

func mkTpath(key path.Path) sessp.Tpath {
	h := fnv.New64a()
	h.Write([]byte(key.String()))
	return sessp.Tpath(h.Sum64())
}

type Obj struct {
	bucket string
	perm   sp.Tperm
	key    path.Path

	// set by fill()
	sz    sp.Tlength
	mtime int64

	// for writing
	ch  chan error
	r   *io.PipeReader
	w   *io.PipeWriter
	off sp.Toffset
}

func makeObj(bucket string, key path.Path, perm sp.Tperm) *Obj {
	o := &Obj{}
	o.bucket = bucket
	o.key = key
	if o.key.Base() == "." {
		o.perm = sp.DMDIR | perm
	} else {
		o.perm = perm
	}
	return o
}

func (o *Obj) String() string {
	return fmt.Sprintf("key '%v' perm %v", o.key, o.perm)
}

func (o *Obj) Size() (sp.Tlength, *serr.Err) {
	return o.sz, nil
}

func (o *Obj) SetSize(sz sp.Tlength) {
	o.sz = sz
}

func (o *Obj) readHead(fss3 *Fss3) *serr.Err {
	key := o.key.String()
	key = toDot(key)
	input := &s3.HeadObjectInput{
		Bucket: &o.bucket,
		Key:    &key,
	}
	result, err := fss3.client.HeadObject(context.TODO(), input)
	if err != nil {
		return serr.MkErrError(err)
	}

	db.DPrintf(db.S3, "readHead: %v %v\n", key, result.ContentLength)
	o.sz = sp.Tlength(result.ContentLength)
	if result.LastModified != nil {
		o.mtime = (*result.LastModified).Unix()
	}
	return nil
}

func makeFsObj(bucket string, perm sp.Tperm, key path.Path) fs.FsObj {
	if perm.IsDir() {
		return makeDir(bucket, key.Copy(), perm)
	} else {
		return makeObj(bucket, key.Copy(), perm)
	}
}

func (o *Obj) fill() *serr.Err {
	if err := o.readHead(fss3); err != nil {
		return err
	}
	return nil
}

// stat without filling
func (o *Obj) stat() *sp.Stat {
	db.DPrintf(db.S3, "stat: %v\n", o)
	name := ""
	if len(o.key) > 0 {
		name = o.key.Base()
	}
	return sp.MkStat(sp.MakeQidPerm(o.perm, 0, o.Path()), o.perm|sp.Tperm(0777), uint32(o.mtime), name, "")
}

func (o *Obj) Path() sessp.Tpath {
	return mkTpath(o.key)
}

// convert ux perms into np perm; maybe symlink?
func (o *Obj) Perm() sp.Tperm {
	return o.perm
}

func (o *Obj) Parent() fs.Dir {
	dir := o.key.Dir()
	return makeDir(o.bucket, dir, sp.DMDIR)
}

func (o *Obj) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	db.DPrintf(db.S3, "Stat: %v\n", o)
	if err := o.fill(); err != nil {
		return nil, err
	}
	st := o.stat()
	st.Length = uint64(o.sz)
	return st, nil
}

// XXX Check permissions?
func (o *Obj) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.S3, "open %v (%T) %v\n", o, o, m)
	if err := o.fill(); err != nil {
		return nil, err
	}
	if m == sp.OWRITE {
		o.setupWriter()
	}
	return o, nil
}

func (o *Obj) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	db.DPrintf(db.S3, "%p: Close %v\n", o, m)
	if m == sp.OWRITE {
		o.w.Close()
		// wait for uploader to finish
		err := <-o.ch
		if err != nil {
			return serr.MkErrError(err)
		}
	}
	return nil
}

func (o *Obj) s3Read(off, cnt int) (io.ReadCloser, sp.Tlength, *serr.Err) {
	key := o.key.String()
	region := ""
	if off != 0 || sp.Tlength(cnt) < o.sz {
		n := off + cnt
		region = "bytes=" + strconv.Itoa(off) + "-" + strconv.Itoa(n-1)
	}
	input := &s3.GetObjectInput{
		Bucket: &o.bucket,
		Key:    &key,
		Range:  &region,
	}
	result, err := fss3.client.GetObject(context.TODO(), input)
	if err != nil {
		return nil, 0, serr.MkErrError(err)
	}
	region1 := ""
	if result.ContentRange != nil {
		region1 = *result.ContentRange
	}
	db.DPrintf(db.S3, "s3Read: %v region %v res %v %v\n", o.key, region, region1, result.ContentLength)
	return result.Body, sp.Tlength(result.ContentLength), nil
}

func (o *Obj) Read(ctx fs.CtxI, off sp.Toffset, cnt sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	db.DPrintf(db.S3, "Read: %v o %v n %v sz %v\n", o.key, off, cnt, o.sz)
	if sp.Tlength(off) >= o.sz {
		return nil, nil
	}
	rdr, n, err := o.s3Read(int(off), int(cnt))
	if err != nil {
		return nil, err
	}
	defer rdr.Close()
	b, error := io.ReadAll(rdr)
	if error != nil {
		db.DPrintf(db.S3, "Read: Read %d err %v\n", n, error)
		return nil, serr.MkErrError(error)
	}
	return b, nil
}

//
// Write using an uploader thread
//

func (o *Obj) setupWriter() {
	db.DPrintf(db.S3, "%p: setupWriter\n", o)
	o.off = 0
	o.ch = make(chan error)
	o.r, o.w = io.Pipe()
	go o.writer(o.ch)
}

func (o *Obj) writer(ch chan error) {
	key := o.key.String()
	uploader := manager.NewUploader(fss3.client)
	_, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: &o.bucket,
		Key:    &key,
		Body:   o.r,
	})
	if err != nil {
		db.DPrintf(db.S3, "Writer %v err %v\n", key, err)
	}
	ch <- err
}

func (o *Obj) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *serr.Err) {
	db.DPrintf(db.S3, "Write %v %v sz %v\n", off, len(b), o.sz)
	if off != o.off {
		db.DPrintf(db.S3, "Write %v err\n", o.off)
		return 0, serr.MkErr(serr.TErrInval, off)
	}
	if n, err := o.w.Write(b); err != nil {
		db.DPrintf(db.S3, "Write %v %v err %v\n", off, len(b), err)
		return 0, serr.MkErrError(err)
	} else {
		o.off += sp.Toffset(n)
		o.SetSize(sp.Tlength(o.off))
		return sessp.Tsize(n), nil
	}
}
