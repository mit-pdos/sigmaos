package fss3

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
)

type Obj struct {
	*info
	perm np.Tperm
	key  np.Path
	ch   chan error
	r    *io.PipeReader
	w    *io.PipeWriter
	off  np.Toffset
}

func makeObj(key np.Path, perm np.Tperm) *Obj {
	o := &Obj{}
	o.key = key
	o.perm = perm
	return o
}

func (o *Obj) String() string {
	if o.info == nil {
		return fmt.Sprintf("key '%v' perm %v", o.key, o.perm)
	} else {
		return fmt.Sprintf("key '%v' perm %v info %v", o.key, o.perm, o.info)
	}
}

func (o *Obj) fillObj() *np.Err {
	if o.info == nil {
		i := cache.lookup(o.key)
		if i != nil {
			o.info = i
			return nil
		}
		i, err := readHead(fss3, o.key)
		if err != nil {
			return err
		}
		o.info = i
		return nil
	}
	return nil
}

func (o *Obj) Qid() np.Tqid {
	return qid(o.perm, o.key)

}

// convert ux perms into np perm; maybe symlink?
func (o *Obj) Perm() np.Tperm {
	return o.perm
}

func (o *Obj) Parent() fs.Dir {
	dir := o.key.Dir()
	return makeDir(dir, np.DMDIR)
}

func (o *Obj) Stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
	db.DPrintf("FSS3", "Stat: %v %p\n", o, o.info)
	if err := o.fillObj(); err != nil {
		return nil, err
	}
	return o.info.stat(), nil
}

// Read object from s3.
func (o *Obj) s3Read(off, cnt int) (io.ReadCloser, np.Tlength, *np.Err) {
	key := o.key.String()
	region := ""
	if off != 0 || np.Tlength(cnt) < o.sz {
		n := off + cnt
		region = "bytes=" + strconv.Itoa(off) + "-" + strconv.Itoa(n-1)
	}
	input := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Range:  &region,
	}
	result, err := fss3.client.GetObject(context.TODO(), input)
	if err != nil {
		return nil, 0, np.MkErrError(err)
	}
	region1 := ""
	if result.ContentRange != nil {
		region1 = *result.ContentRange
	}
	db.DPrintf("FSS3", "s3Read: region %v res %v %v\n", region, region1, result.ContentLength)
	return result.Body, np.Tlength(result.ContentLength), nil
}

// XXX Check permissions?
func (o *Obj) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {
	if err := o.fillObj(); err != nil {
		return nil, err
	}
	if m == np.OREAD {
		o.setupReader()
	}
	if m == np.OWRITE {
		o.setupWriter()
	}
	return nil, nil
}

func (o *Obj) Close(ctx fs.CtxI, m np.Tmode) *np.Err {
	db.DPrintf("FSS3", "%p: Close %v\n", o, m)
	if m == np.OWRITE {
		o.w.Close()
		// wait for writer to finish
		err := <-o.ch
		if err != nil {
			return np.MkErrError(err)
		}
	}
	return nil
}

// XXX what if file has grown or shrunk?
func (o *Obj) setupReader() {
	db.DPrintf("FSS3", "%p: setupReader\n", o)
	o.off = 0
}

func (o *Obj) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	db.DPrintf("FSS3", "Read: %v %v %v %v\n", o.key, off, cnt, o.sz)
	if np.Tlength(off) >= o.sz {
		return nil, nil
	}
	rdr, n, err := o.s3Read(int(off), int(cnt))
	if err != nil {
		return nil, err
	}
	defer rdr.Close()
	b, error := io.ReadAll(rdr)
	if error != nil {
		db.DPrintf("FSS3", "Read: Read %d err %v\n", n, error)
		return nil, np.MkErrError(error)
	}
	return b, nil
}

// func (o *Obj) reader() {
// 	key := o.key.String()
// 	downloader := manager.NewDownloaderWithClient(o.fss3.client)
// 	_, err := downloader.Download(??, &s3.GetObjectInput{
// 		Bucket: &bucket,
// 		Key:    &key,
// 		Body:   o.r,
// 	})
// 	if err != nil {
// 		db.DPrintf("FSS3", "reader %v err %v\n", key, err)
// 	}
// }

func (o *Obj) setupWriter() {
	db.DPrintf("FSS3", "%p: setupWriter\n", o)
	o.off = 0
	o.ch = make(chan error)
	o.r, o.w = io.Pipe()
	go o.writer(o.ch)
}

func (o *Obj) writer(ch chan error) {
	key := o.key.String()
	uploader := manager.NewUploader(fss3.client)
	_, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   o.r,
	})
	if err != nil {
		db.DPrintf("FSS3", "Writer %v err %v\n", key, err)
	}
	ch <- err
}

func (o *Obj) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	db.DPrintf("FSS3", "Write %v %v sz %v\n", off, len(b), o.sz)
	if off != o.off {
		db.DPrintf("FSS3", "Write %v err\n", o.off)
		return 0, np.MkErr(np.TErrInval, off)
	}
	if n, err := o.w.Write(b); err != nil {
		db.DPrintf("FSS3", "Write %v %v err %v\n", off, len(b), err)
		return 0, np.MkErrError(err)
	} else {
		o.off += np.Toffset(n)
		o.sz = np.Tlength(o.off)
		return np.Tsize(n), nil
	}
}
