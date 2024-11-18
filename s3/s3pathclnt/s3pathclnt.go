package s3pathclnt

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/netproxyclnt"
	"sigmaos/path"
	"sigmaos/serr"
	sos "sigmaos/sigmaos"
	sp "sigmaos/sigmap"
	"sigmaos/util/syncmap"
)

type S3PathClnt struct {
	s3clnt *s3.Client
	rfids  *syncmap.SyncMap[sp.Tfid, *s3Reader]
	wfids  *syncmap.SyncMap[sp.Tfid, *s3Writer]

	sync.Mutex
	next sp.Tfid
}

func NewS3PathClnt(s3secrets *sp.SecretProto, npc *netproxyclnt.NetProxyClnt) (*S3PathClnt, error) {
	s3c := &S3PathClnt{
		rfids: syncmap.NewSyncMap[sp.Tfid, *s3Reader](),
		wfids: syncmap.NewSyncMap[sp.Tfid, *s3Writer](),
	}
	if s3clnt, err := getS3Client(s3secrets, npc); err != nil {
		return nil, err
	} else {
		s3c.s3clnt = s3clnt
	}
	return s3c, nil
}

func (s3c *S3PathClnt) allocFid() sp.Tfid {
	s3c.Lock()
	defer s3c.Unlock()
	fid := s3c.next
	s3c.next += 1
	return fid
}

func (s3c *S3PathClnt) Open(pn string, principal *sp.Tprincipal, mode sp.Tmode, w sos.Watch) (sp.Tfid, error) {
	if w != nil {
		return sp.NoFid, serr.NewErr(serr.TErrNotSupported, "Twait")
	}
	if mode == sp.OREAD {
		s3r, err := s3c.openS3Reader(pn)
		if err != nil {
			return sp.NoFid, err
		}
		fid := s3c.allocFid()
		s3c.rfids.Insert(fid, s3r)
		db.DPrintf(db.S3CLNT, "Open %v %v %v", pn, mode, fid)
		return fid, nil
	}
	return sp.NoFid, serr.NewErr(serr.TErrInval, mode)
}

func (s3c *S3PathClnt) Create(pn string, principal *sp.Tprincipal, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f *sp.Tfence) (sp.Tfid, error) {
	s3w, err := s3c.openS3Writer(pn)
	if err != nil {
		return sp.NoFid, err
	}
	fid := s3c.allocFid()
	s3c.wfids.Insert(fid, s3w)
	db.DPrintf(db.S3CLNT, "Create %v %v", pn, fid)
	return fid, nil
}

func (s3c *S3PathClnt) ReadF(fid sp.Tfid, off sp.Toffset, b []byte, f *sp.Tfence) (sp.Tsize, error) {
	s3r, ok := s3c.rfids.Lookup(fid)
	if !ok {
		return 0, serr.NewErr(serr.TErrNotfound, fid)
	}
	n, err := s3r.read(off, b)
	return sp.Tsize(n), err
}

func (s3c *S3PathClnt) PreadRdr(fid sp.Tfid, off sp.Toffset, sz sp.Tsize) (io.ReadCloser, error) {
	s3r, ok := s3c.rfids.Lookup(fid)
	if !ok {
		return nil, serr.NewErr(serr.TErrNotfound, fid)
	}
	return s3r.readRdr(off, sz)
}

func (s3c *S3PathClnt) WriteF(fid sp.Tfid, off sp.Toffset, data []byte, f *sp.Tfence) (sp.Tsize, error) {
	s3w, ok := s3c.wfids.Lookup(fid)
	if !ok {
		return 0, serr.NewErr(serr.TErrNotfound, fid)
	}
	n, err := s3w.write(off, data)
	return sp.Tsize(n), err
}

func (s3c *S3PathClnt) Clunk(fid sp.Tfid) error {
	db.DPrintf(db.S3CLNT, "Clunk %v", fid)
	s3r, ok := s3c.rfids.Lookup(fid)
	if ok {
		s3r.close()
		return nil
	}
	s3w, ok := s3c.wfids.Lookup(fid)
	if ok {
		s3w.close()
		return nil
	}
	return serr.NewErr(serr.TErrNotfound, fid)
}

// XXX deduplicate with s3
func getS3Client(s3secrets *sp.SecretProto, npc *netproxyclnt.NetProxyClnt) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithCredentialsProvider(
			auth.NewAWSCredentialsProvider(s3secrets),
		),
		config.WithRegion(s3secrets.Metadata),
	)
	if err != nil {
		db.DFatalf("Failed to load SDK configuration %v", err)
	}
	clnt := s3.NewFromConfig(cfg, func(o *s3.Options) {
		hclnt := awshttp.NewBuildableClient().WithTransportOptions(func(t *http.Transport) {
			t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				a, err := sp.NewTaddrFromString(addr, sp.OUTER_CONTAINER_IP)
				if err != nil {
					return nil, err
				}
				return npc.Dial(sp.NewEndpoint(sp.EXTERNAL_EP, []*sp.Taddr{a}))
			}
		})
		o.HTTPClient = hclnt
		o.UsePathStyle = true
	})
	return clnt, nil
}

func (s3c *S3PathClnt) S3Stat(bucket, key string) (sp.Tlength, error) {
	input := &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	result, err := s3c.s3clnt.HeadObject(context.TODO(), input)
	if err != nil {
		db.DPrintf(db.S3, "readHead: %v err %v\n", key, err)
		return 0, serr.NewErrError(err)
	}
	db.DPrintf(db.S3, "readHead: %v %v %v\n", key, result.ContentLength, err)
	return sp.Tlength(*result.ContentLength), nil
}

func (s3c *S3PathClnt) openS3Reader(pn string) (*s3Reader, error) {
	pn0, _ := strings.CutPrefix(pn, sp.S3CLNT+"/")
	p := path.Split(pn0)

	bucket := p[0]
	key := p[1:].String()

	db.DPrintf(db.S3CLNT, "openS3Reader %v: bucket %q key %q", pn, bucket, key)

	sz, err := s3c.S3Stat(bucket, key)
	if err != nil {
		return nil, err
	}

	db.DPrintf(db.S3CLNT, "openS3Reader: S3Stat sz %v", sz)

	reader := &s3Reader{
		clnt:   s3c.s3clnt,
		bucket: bucket,
		key:    key,
		sz:     sz,
	}
	return reader, err
}

func (s3c *S3PathClnt) openS3Writer(pn string) (*s3Writer, error) {
	pn0, _ := strings.CutPrefix(pn, sp.S3CLNT+"/")
	p := path.Split(pn0)

	bucket := p[0]
	key := p[1:].String()

	db.DPrintf(db.S3CLNT, "openS3Writer %v: bucket %q key %q", pn, bucket, key)

	writer := &s3Writer{
		clnt:   s3c.s3clnt,
		bucket: bucket,
		key:    key,
		ch:     make(chan error),
	}
	go writer.writer()
	return writer, nil
}
