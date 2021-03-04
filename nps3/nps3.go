package nps3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/npsrv"
)

type Nps3 struct {
	mu     sync.Mutex
	srv    *npsrv.NpServer
	client *s3.Client
	nextId np.Tpath // XXX delete?
	ch     chan bool
}

func MakeNps3() *Nps3 {
	nps3 := &Nps3{}
	nps3.ch = make(chan bool)
	db.SetDebug(false)
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile("me-mit"))
	if err != nil {
		log.Fatalf("Failed to load SDK configuration %v", err)
	}

	nps3.client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("LocalIP %v %v\n", fslib.S3, err)
	}
	nps3.srv = npsrv.MakeNpServer(nps3, ip+":0")
	fsl := fslib.MakeFsLib("s3")
	err = fsl.PostServiceUnion(nps3.srv.MyAddr(), fslib.S3, nps3.srv.MyAddr())
	if err != nil {
		log.Fatalf("PostServiceUnion failed %v %v\n", nps3.srv.MyAddr(), err)
	}

	return nps3
}

func (nps3 *Nps3) Serve() {
	<-nps3.ch
}

func (nps3 *Nps3) done() {
	nps3.ch <- true
}

func mode(key string) np.Tperm {
	m := np.Tperm(0)
	if key == "" || strings.HasSuffix(key, "/") {
		m = np.DMDIR
	}
	return m
}

func (nps3 *Nps3) makeObjL(key []string, t np.Tperm, p *Obj) *Obj {
	id := nps3.nextId
	nps3.nextId += 1

	o := &Obj{}
	o.nps3 = nps3
	o.key = key
	o.t = t
	o.id = id
	o.dirents = make(map[string]*Obj)
	o.parent = p
	return o
}

func (nps3 *Nps3) makeObj(key []string, t np.Tperm, p *Obj) *Obj {
	nps3.mu.Lock()
	defer nps3.mu.Unlock()
	return nps3.makeObjL(key, t, p)
}

type Obj struct {
	nps3    *Nps3
	key     []string
	t       np.Tperm
	id      np.Tpath
	sz      np.Tlength
	mtime   time.Time
	dirents map[string]*Obj
	parent  *Obj
}

func (o *Obj) String() string {
	s := fmt.Sprintf("%v %v %v %v %v", o.key, o.t, o.id, o.sz, o.mtime)
	return s
}

func (o *Obj) qid() np.Tqid {
	return np.MakeQid(np.Qtype(o.t>>np.QTYPESHIFT),
		np.TQversion(0), np.Tpath(o.id))
}

// if o.key is prefix of key, include next component of key (unless
// we already seen it
func (o *Obj) includeName(key string) (string, np.Tperm, bool) {
	s := np.Split(key)
	m := np.Tperm(0)
	db.DPrintf("s %v o.key %v dirents %v\n", s, o.key, o.dirents)
	for i, c := range o.key {
		if c != s[i] {
			return "", m, false
		}
	}
	if len(s) == len(o.key) {
		return "", m, false
	}
	name := s[len(o.key)]
	_, ok := o.dirents[name]
	if ok {
		m = o.t
	} else {
		if len(s) > len(o.key)+1 {
			m = np.DMDIR
		}
	}
	return name, m, !ok
}

func (o *Obj) stat() *np.Stat {
	st := &np.Stat{}
	if len(o.key) > 0 {
		st.Name = o.key[len(o.key)-1]
	} else {
		st.Name = "" // root
	}
	st.Mode = o.t | np.Tperm(0777)
	st.Qid = o.qid()
	st.Uid = ""
	st.Gid = ""
	st.Length = o.sz
	st.Mtime = uint32(o.mtime.Unix())
	return st
}

func (o *Obj) readHead() error {
	key := np.Join(o.key)
	input := &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	result, err := o.nps3.client.HeadObject(context.TODO(), input)
	if err != nil {
		return err
	}
	o.sz = np.Tlength(result.ContentLength)
	if result.LastModified != nil {
		o.mtime = *result.LastModified
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
	result, err := o.nps3.client.GetObject(context.TODO(), input)
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
				o.sz = np.Tlength(l)
			}
		}
	}
	return result.Body, nil
}

func (o *Obj) readFile(off, cnt int) ([]byte, error) {
	db.DPrintf("readFile: %v %v %v\n", o.key, off, cnt)

	// XXX what if file has grown or shrunk? is contentRange (see below) reliable?
	if o.sz == 0 {
		o.readHead()
	}
	if np.Tlength(off) >= o.sz {
		return nil, nil
	}
	r, err := o.s3Read(off, cnt)
	if err != nil {
		return nil, err
	}
	var b []byte
	for cnt > 0 {
		p := make([]byte, CHUNKSZ)
		n, err := r.Read(p)
		if n > 0 {
			// in case s3 returns more than we asked for
			if n > cnt {
				n = cnt
			}
			b = append(b, p[:n]...)
			off += n
			cnt -= n
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

func (o *Obj) s3ReadDir() error {
	key := np.Join(o.key)
	maxKeys := 0
	params := &s3.ListObjectsV2Input{
		Bucket: &bucket,
		Prefix: &key,
	}
	p := s3.NewListObjectsV2Paginator(o.nps3.client, params,
		func(o *s3.ListObjectsV2PaginatorOptions) {
			if v := int32(maxKeys); v != 0 {
				o.Limit = v
			}
		})
	for p.HasMorePages() {
		page, err := p.NextPage(context.TODO())
		if err != nil {
			return fmt.Errorf("bad offset")
		}
		for _, obj := range page.Contents {
			db.DPrintf("Key: %v\n", *obj.Key)
			if n, m, ok := o.includeName(*obj.Key); ok {
				db.DPrintf("incl %v\n", n)
				o1 := o.nps3.makeObj(append(o.key, n), m, o)
				o.dirents[n] = o1
			}
		}
	}
	return nil
}

func (o *Obj) lookup(p []string) (*Obj, error) {
	db.DPrintf("%v: lookup %v\n", o, p)
	if o.t != np.DMDIR {
		return nil, fmt.Errorf("Not a directory")
	}
	_, err := o.readDir()
	if err != nil {
		return nil, err
	}
	o1, ok := o.dirents[p[0]]
	if !ok {
		return nil, fmt.Errorf("file not found")
	}
	if len(p) == 1 {
		return o1, nil
	} else {
		return o1.lookup(p[1:])
	}
}

func (o *Obj) readDir() ([]*np.Stat, error) {
	var dirents []*np.Stat
	db.DPrintf("readDir: %v\n", o)
	if len(o.dirents) == 0 {
		o.s3ReadDir()

	}
	for _, o1 := range o.dirents {
		dirents = append(dirents, o1.stat())
	}
	o.sz = npcodec.DirSize(dirents)
	return dirents, nil
}

func (o *Obj) Create(name string, perm np.Tperm, m np.Tmode) (*Obj, error) {
	key := np.Join(append(o.key, name))
	input := &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	_, err := o.nps3.client.PutObject(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	// XXX ignored perm, only files not directories
	o1 := o.nps3.makeObj(np.Split(key), 0, o)
	o.dirents[name] = o1
	return o1, nil
}

func (o *Obj) Remove() error {
	key := np.Join(o.key)
	input := &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	_, err := o.nps3.client.DeleteObject(context.TODO(), input)
	if err != nil {
		return err
	}
	delete(o.parent.dirents, o.key[len(o.key)-1])
	return nil
}

// XXX maybe represent a file as several objects to avoid
// reading the whole file to update it.
// XXX maybe buffer all writes before writing to S3 (on clunk?)
func (o *Obj) writeFile(off int, b []byte) (int, error) {
	db.DPrintf("writeFile %v %v sz %v\n", off, len(b), o.sz)
	key := np.Join(o.key)
	r, err := o.s3Read(-1, 0)
	if err != nil {
		return 0, err
	}
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return 0, err
	}
	if off < len(data) { // prefix of data?
		b1 := append(data[:off], b...)
		if off+len(b) < len(data) { // suffix of data?
			b = append(b1, data[off+len(b):]...)
		}
	} else if off == len(data) { // append?
		b = append(data, b...)
	} else { // off > len(data), a hole
		hole := make([]byte, off-len(data))
		b1 := append(data, hole...)
		b = append(b1, b...)
	}
	r1 := bytes.NewReader(b)
	input := &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   r1,
	}
	_, err = o.nps3.client.PutObject(context.TODO(), input)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}
