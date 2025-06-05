package fss3

import (
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func newTpath(key path.Tpathname) sp.Tpath {
	h := fnv.New64a()
	h.Write([]byte(key.String()))
	return sp.Tpath(h.Sum64())
}

type Obj struct {
	bucket string
	perm   sp.Tperm
	key    path.Tpathname

	// set by fill()
	sz    sp.Tlength
	mtime int64

	// for writing
	ch  chan error
	r   *io.PipeReader
	w   *io.PipeWriter
	off sp.Toffset
}

func newObj(bucket string, key path.Tpathname, perm sp.Tperm) *Obj {
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
	return fmt.Sprintf("{Obj bucket %q key %q perm %v}", o.bucket, o.key, o.perm)
}

func (o *Obj) readHead(ctx fs.CtxI, fss3 *Fss3) *serr.Err {
	key := o.key.String()
	key = toDot(key)
	input := &s3.HeadObjectInput{
		Bucket: &o.bucket,
		Key:    &key,
	}
	clnt, err1 := fss3.getClient(ctx)
	if err1 != nil {
		return err1
	}
	result, err := clnt.HeadObject(context.TODO(), input)
	if err != nil {
		db.DPrintf(db.S3, "readHead: %v err %v\n", key, err)
		return serr.NewErrError(err)
	}
	db.DPrintf(db.S3, "readHead: %v %v %v\n", key, result.ContentLength, err)
	o.sz = sp.Tlength(*result.ContentLength)
	if result.LastModified != nil {
		o.mtime = (*result.LastModified).Unix()
	}
	return nil
}

func newFsObj(bucket string, key path.Tpathname, perm sp.Tperm) fs.FsObj {
	if perm.IsDir() {
		return newDir(bucket, key.Copy(), perm)
	} else {
		return newObj(bucket, key.Copy(), perm)
	}
}

func (o *Obj) fill(ctx fs.CtxI) *serr.Err {
	if err := o.readHead(ctx, fss3); err != nil {
		return err
	}
	return nil
}

// stat without filling
func (o *Obj) NewStat() (*sp.Tstat, *serr.Err) {
	db.DPrintf(db.S3, "NewStat: %v\n", o)
	name := ""
	if len(o.key) > 0 {
		name = o.key.Base()
	}
	return sp.NewStat(sp.NewQidPerm(o.perm, 0, o.Path()), o.perm|sp.Tperm(0777), uint32(o.mtime), name, ""), nil
}

func (o *Obj) Path() sp.Tpath {
	p := path.Tpathname{o.bucket}
	return newTpath(p.AppendPath(o.key))
}

func (o *Obj) Dev() sp.Tdev {
	return sp.DEV_S3FS
}

func (o *Obj) Perm() sp.Tperm {
	return o.perm
}

func (o *Obj) IsLeased() bool {
	return false
}

func (o *Obj) Stat(ctx fs.CtxI) (*sp.Tstat, *serr.Err) {
	db.DPrintf(db.S3, "Stat: %v\n", o)
	if err := o.fill(ctx); err != nil {
		db.DPrintf(db.S3, "Stat: %v err %v\n", o, err)
		return nil, err
	}
	st, err := o.NewStat()
	if err != nil {
		return nil, err
	}
	st.SetLength(o.sz)
	return st, nil
}

// XXX Check permissions?
func (o *Obj) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.S3, "open %v (%T) %v\n", o, o, m)
	if err := o.fill(ctx); err != nil {
		return nil, err
	}
	if m == sp.OWRITE {
		o.setupWriter(ctx)
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
			return serr.NewErrError(err)
		}
	}
	return nil
}

func (o *Obj) s3Read(ctx fs.CtxI, off, cnt int) (io.ReadCloser, sp.Tlength, *serr.Err) {
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
	clnt, err1 := fss3.getClient(ctx)
	if err1 != nil {
		return nil, 0, err1
	}
	result, err := clnt.GetObject(context.TODO(), input)
	if err != nil {
		return nil, 0, serr.NewErrError(err)
	}
	region1 := ""
	if result.ContentRange != nil {
		region1 = *result.ContentRange
	}
	db.DPrintf(db.S3, "s3Read: %v region %v res %v %v\n", o.key, region, region1, result.ContentLength)
	return result.Body, sp.Tlength(*result.ContentLength), nil
}

func (o *Obj) Read(ctx fs.CtxI, off sp.Toffset, cnt sp.Tsize, f sp.Tfence) ([]byte, *serr.Err) {
	db.DPrintf(db.S3, "Read: %v o %v n %v sz %v\n", o.key, off, cnt, o.sz)
	if sp.Tlength(off) >= o.sz {
		return nil, nil
	}
	rdr, n, err := o.s3Read(ctx, int(off), int(cnt))
	if err != nil {
		return nil, err
	}
	defer rdr.Close()
	b, error := io.ReadAll(rdr)
	if error != nil {
		db.DPrintf(db.S3, "Read: Read %d err %v\n", n, error)
		return nil, serr.NewErrError(error)
	}
	return b, nil
}

func (o *Obj) s3Create(ctx fs.CtxI) *serr.Err {
	key := o.key.String()
	input := &s3.PutObjectInput{
		Bucket: &o.bucket,
		Key:    &key,
	}
	clnt, err1 := fss3.getClient(ctx)
	if err1 != nil {
		return err1
	}
	if _, err := clnt.PutObject(context.TODO(), input); err != nil {
		return serr.NewErrError(err)
	}
	return nil
}

//
// Write using an uploader thread
//

func (o *Obj) setupWriter(ctx fs.CtxI) {
	db.DPrintf(db.S3, "%p: setupWriter\n", o)
	o.off = 0
	o.ch = make(chan error)
	o.r, o.w = io.Pipe()
	go o.writer(ctx, o.ch)
}

func (o *Obj) writer(ctx fs.CtxI, ch chan error) {
	key := o.key.String()
	clnt, err1 := fss3.getClient(ctx)
	if err1 != nil {
		db.DPrintf(db.ERROR, "Error getclient: %v", err1)
		return
	}
	uploader := manager.NewUploader(clnt)
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

func (o *Obj) Write(ctx fs.CtxI, off sp.Toffset, b []byte, f sp.Tfence) (sp.Tsize, *serr.Err) {
	db.DPrintf(db.S3, "Write %v %v sz %v f %v\n", off, len(b), o.sz, f)
	if off != o.off {
		db.DPrintf(db.S3, "Write %v err\n", o.off)
		return 0, serr.NewErr(serr.TErrInval, off)
	}
	if n, err := o.w.Write(b); err != nil {
		db.DPrintf(db.S3, "Write %v %v err %v\n", off, len(b), err)
		return 0, serr.NewErrError(err)
	} else {
		o.off += sp.Toffset(n)
		o.sz = sp.Tlength(o.off)
		return sp.Tsize(n), nil
	}
}

// ===== The following functions are needed to make an s3 obj support fs.Inode

func (o *Obj) Unlink() {
	db.DFatalf("Unimplemented")
}

func (o *Obj) SetMtime(mtime int64) {
	db.DFatalf("Unimplemented")
}

func (o *Obj) Mtime() int64 {
	db.DFatalf("Unimplemented")
	return 0
}
