package s3pathclnt

import (
	"context"
	"net"
	"net/http"
	"strings"

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
)

const (
	MB      = (1 << 20)
	CHUNKSZ = 4 * MB
)

type S3PathClnt struct {
	s3clnt *s3.Client
	s3r    *s3Reader
	s3w    *s3Writer
}

func NewS3PathClnt(s3secrets *sp.SecretProto, npc *netproxyclnt.NetProxyClnt) (*S3PathClnt, error) {
	s3c := &S3PathClnt{}
	if s3clnt, err := getS3Client(s3secrets, npc); err != nil {
		return nil, err
	} else {
		s3c.s3clnt = s3clnt
	}
	return s3c, nil
}

func (s3c *S3PathClnt) Open(pn string, principal *sp.Tprincipal, mode sp.Tmode, w sos.Watch) (sp.Tfid, error) {
	db.DPrintf(db.S3CLNT, "Open %v %v", pn, mode)
	if w != nil {
		return sp.NoFid, serr.NewErr(serr.TErrNotSupported, "Twait")
	}
	if mode == sp.OREAD {
		s3r, err := s3c.openS3Reader(pn)
		if err != nil {
			return sp.NoFid, err
		}
		s3c.s3r = s3r
	}
	return 0, nil
}

func (s3c *S3PathClnt) ReadF(fid sp.Tfid, off sp.Toffset, b []byte, f *sp.Tfence) (sp.Tsize, error) {
	n, err := s3c.s3r.read(off, b)
	return sp.Tsize(n), err
}

func (s3c *S3PathClnt) WriteF(fid sp.Tfid, off sp.Toffset, data []byte, f *sp.Tfence) (sp.Tsize, error) {
	return 0, nil
}

func (s3c *S3PathClnt) Clunk(fid sp.Tfid) error {
	// XXX in case of write wait
	return nil
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

func (s3c *S3PathClnt) openS3Writer(pn string) (int, error) {
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
	s3c.s3w = writer
	return 0, nil
}
