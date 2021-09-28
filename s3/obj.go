package fss3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
)

func mode(key string) np.Tperm {
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
	key    []string
	sz     np.Tlength
	isRead bool
}

func (fss3 *Fss3) makeObj(key []string, t np.Tperm, d *Dir) fs.FsObj {
	o := &Obj{}
	o.Inode = inode.MakeInode("", t, d)
	o.fss3 = fss3
	o.key = key
	return o
}

func (o *Obj) String() string {
	s := fmt.Sprintf("%v t %v id %v sz %v %v", o.key, o.Perm(), o.Inum(), o.sz,
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

func (o *Obj) Stat(ctx fs.CtxI) (*np.Stat, error) {
	db.DLPrintf("FSS3", "Stat: %v\n", o)
	var err error
	o.mu.Lock()
	read := o.isRead
	o.mu.Unlock()
	if !read {
		err = o.readHead()
	}
	return o.stat(), err
}

func (o *Obj) readHead() error {
	key := np.Join(o.key)
	input := &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	result, err := o.fss3.client.HeadObject(context.TODO(), input)
	if err != nil {
		return err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sz = np.Tlength(result.ContentLength)
	if result.LastModified != nil {
		o.SetMtime((*result.LastModified).Unix())
	}
	return nil
}

// Read object from s3. If off == -1, read whole object; otherwise,
// read a region.
func (o *Obj) s3Read(off, cnt int) (io.ReadCloser, error) {
	key := np.Join(o.key)
	region := ""
	if off != -1 {
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
		return nil, err
	}
	// Check if contentRange, lists the length of the object, and perhaps
	// update the length we know about.
	if result.ContentRange != nil {
		r := strings.Split(*result.ContentRange, "/")
		if len(r) > 1 {
			l, err := strconv.Atoi(r[1])
			if err == nil {
				o.mu.Lock()
				defer o.mu.Unlock()
				o.sz = np.Tlength(l)
			}
		}
	}
	return result.Body, nil
}

func (o *Obj) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, error) {
	db.DLPrintf("FSS3", "Read: %v %v %v\n", o.key, off, cnt)
	// XXX what if file has grown or shrunk? is contentRange (see below) reliable?
	if !o.isRead {
		o.readHead()
	}
	if np.Tlength(off) >= o.sz {
		return nil, nil
	}
	r, err := o.s3Read(int(off), int(cnt))
	if err != nil {
		return nil, err
	}
	var b []byte
	for cnt > 0 {
		p := make([]byte, CHUNKSZ)
		n, err := r.Read(p)
		if n > 0 {
			// in case s3 returns more than we asked for
			if n > int(cnt) {
				n = int(cnt)
			}
			b = append(b, p[:n]...)
			off += np.Toffset(n)
			cnt -= np.Tsize(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

// XXX Check permissions?
func (o *Obj) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, error) {
	return nil, nil
}

func (o *Obj) Close(ctx fs.CtxI, m np.Tmode) error {
	return nil
}

// XXX maybe represent a file as several objects to avoid
// reading the whole file to update it.
// XXX maybe buffer all writes before writing to S3 (on clunk?)
func (o *Obj) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	db.DLPrintf("FSS3", "Write %v %v sz %v\n", off, len(b), o.sz)
	key := np.Join(o.key)
	r, err := o.s3Read(-1, 0)
	if err != nil {
		return 0, err
	}
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return 0, err
	}
	if int(off) < len(data) { // prefix of data?
		b1 := append(data[:off], b...)
		if int(off)+len(b) < len(data) { // suffix of data?
			b = append(b1, data[int(off)+len(b):]...)
		}
	} else if int(off) == len(data) { // append?
		b = append(data, b...)
	} else { // off > len(data), a hole
		hole := make([]byte, int(off)-len(data))
		b1 := append(data, hole...)
		b = append(b1, b...)
	}
	r1 := bytes.NewReader(b)
	input := &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   r1,
	}
	_, err = o.fss3.client.PutObject(context.TODO(), input)
	if err != nil {
		return 0, err
	}
	return np.Tsize(len(b)), nil
}
