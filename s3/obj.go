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

	// for writing
	ch  chan error
	r   *io.PipeReader
	w   *io.PipeWriter
	off np.Toffset

	// for reading
	buff *writeAtBuffer
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

func (o *Obj) fill() *np.Err {
	if o.info == nil {
		i := cache.lookup(o.key)
		if i != nil {
			o.info = i
			return nil
		}
		var err *np.Err
		if o.perm.IsDir() {
			i, err = s3ReadDirL(fss3, o.key)
		} else {
			i, err = readHead(fss3, o.key)
		}
		if err != nil {
			return err
		}
		o.info = i
		return nil
	}
	return nil
}

func (o *Obj) Qid() np.Tqid {
	q := qid(o.perm, o.key)
	return q
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
	if err := o.fill(); err != nil {
		return nil, err
	}
	return o.info.stat(), nil
}

// XXX Check permissions?
func (o *Obj) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {
	db.DPrintf("FSS3", "open %v (%T) %v\n", o, o, m)
	if err := o.fill(); err != nil {
		return nil, err
	}
	//if m == np.OREAD {
	//	o.setupReader()
	//}
	if m == np.OWRITE {
		o.setupWriter()
	}
	return o, nil
}

func (o *Obj) Close(ctx fs.CtxI, m np.Tmode) *np.Err {
	db.DPrintf("FSS3", "%p: Close %v\n", o, m)
	if m == np.OWRITE {
		o.w.Close()
		// wait for uploader to finish
		err := <-o.ch
		if err != nil {
			return np.MkErrError(err)
		}
	}
	return nil
}

//
// Read using downloader thread and writeAtBuffer
//

func (o *Obj) setupReader() {
	db.DPrintf("FSS3", "%p: setupReader\n", o)
	o.buff = mkWriteAtBuffer(o.sz)
	go o.reader()
}

func (o *Obj) reader() {
	key := o.key.String()
	downloader := manager.NewDownloader(fss3.client)
	_, err := downloader.Download(context.TODO(), o.buff, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		db.DPrintf("FSS3", "reader %v err %v\n", key, err)
		o.buff.setErr(err)
	}
}

func (o *Obj) Read0(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	db.DPrintf("FSS3", "Read: %v %v %v %v\n", o.key, off, cnt, o.Size())
	if np.Tlength(off) >= o.Size() {
		return nil, nil
	}
	if np.Tlength(off)+np.Tlength(cnt) > o.Size() {
		cnt = np.Tsize(o.Size()) - np.Tsize(off)
	}
	return o.buff.read(off, cnt)
}

//
// Old read implementation around in case we need to read
// small parts of a file instead of the complete file.
//

func (o *Obj) s3Read(off, cnt int) (io.ReadCloser, np.Tlength, *np.Err) {
	key := o.key.String()
	region := ""
	if off != 0 || np.Tlength(cnt) < o.Size() {
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
	db.DPrintf("FSS3", "s3Read: %v region %v res %v %v\n", o.key, region, region1, result.ContentLength)
	return result.Body, np.Tlength(result.ContentLength), nil
}

func (o *Obj) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	db.DPrintf("FSS3", "Read: %v o %v n %v sz %v\n", o.key, off, cnt, o.Size())
	if np.Tlength(off) >= o.Size() {
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

//
// Write using an uploader thread
//

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
	db.DPrintf("FSS3", "Write %v %v sz %v\n", off, len(b), o.Size())
	if off != o.off {
		db.DPrintf("FSS3", "Write %v err\n", o.off)
		return 0, np.MkErr(np.TErrInval, off)
	}
	if n, err := o.w.Write(b); err != nil {
		db.DPrintf("FSS3", "Write %v %v err %v\n", off, len(b), err)
		return 0, np.MkErrError(err)
	} else {
		o.off += np.Toffset(n)
		o.SetSize(np.Tlength(o.off))
		return np.Tsize(n), nil
	}
}
