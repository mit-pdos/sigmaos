package fss3

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
)

func perm(key string) np.Tperm {
	m := np.Tperm(0)
	if key == "" || strings.HasSuffix(key, "/") {
		m = np.DMDIR
	}
	return m
}

type Obj struct {
	*inode.Inode
	mu     sync.Mutex
	fss3   *Fss3
	key    np.Path
	sz     np.Tlength
	r      *io.PipeReader
	w      *io.PipeWriter
	off    np.Toffset
	isRead bool
}

func (fss3 *Fss3) makeObj(key np.Path, t np.Tperm, d *Dir) *Obj {
	o := &Obj{}
	o.Inode = inode.MakeInode(nil, t, d)
	o.fss3 = fss3
	o.key = key
	return o
}

func (o *Obj) String() string {
	s := fmt.Sprintf("%v t %v id %v sz %v %v", o.key, o.Perm(), o.Qid(), o.sz,
		o.Mtime())
	return s
}

func (o *Obj) Size() np.Tlength {
	return o.sz
}

func (o *Obj) stat() *np.Stat {
	st := &np.Stat{}
	if len(o.key) > 0 {
		st.Name = o.key[len(o.key)-1]
	} else {
		st.Name = "" // root
	}
	st.Mode = o.Perm() | np.Tperm(0777)
	st.Qid = o.Qid()
	st.Uid = ""
	st.Gid = ""
	st.Length = o.sz
	st.Mtime = uint32(o.Mtime())
	return st
}

func (o *Obj) Stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
	db.DPrintf("FSS3", "Stat: %v %v\n", o, o.isRead)
	var err *np.Err
	o.mu.Lock()
	read := o.isRead
	o.mu.Unlock()
	if !read {
		err = o.readHead()
	}
	return o.stat(), err
}

func (o *Obj) readHead() *np.Err {
	key := o.key.String()
	input := &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	result, err := o.fss3.client.HeadObject(context.TODO(), input)
	if err != nil {
		return np.MkErrError(err)
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	db.DPrintf("FSS3", "readHead: %v %v\n", o.key.String(), result.ContentLength)

	o.sz = np.Tlength(result.ContentLength)
	if result.LastModified != nil {
		o.SetMtime((*result.LastModified).Unix())
	}
	return nil
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
	result, err := o.fss3.client.GetObject(context.TODO(), input)
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

// XXX Check permissions?
func (o *Obj) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {
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
	}
	return nil
}

// XXX what if file has grown or shrunk? is contentRange (see below) reliable?
func (o *Obj) setupReader() {
	db.DPrintf("FSS3", "%p: setupReader\n", o)
	o.readHead()
	o.off = 0
	// o.r, o.w = io.Pipe()
	// go o.writer()
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
	o.r, o.w = io.Pipe()
	go o.writer()
}

func (o *Obj) writer() {
	key := o.key.String()
	uploader := manager.NewUploader(o.fss3.client)
	_, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   o.r,
	})
	if err != nil {
		db.DPrintf("FSS3", "Writer %v err %v\n", key, err)
	}
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
