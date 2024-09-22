package fslib

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	// "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

const (
	MB      = 1 << 20
	CHUNKSZ = 4 * MB
)

type s3Reader struct {
	clnt      *s3.Client
	rdr       io.ReadCloser
	bucket    string
	key       string
	offset    sp.Toffset
	chunkSize int64
	sz        sp.Tlength
	n         sp.Tlength
}

func (s3rdr *s3Reader) s3Read(off, cnt uint64) (io.ReadCloser, sp.Tlength, error) {
	db.DPrintf(db.S3, "s3Read %d %d", off, cnt)
	key := s3rdr.key
	region := ""
	if off != 0 || sp.Tlength(cnt) < s3rdr.sz {
		n := off + cnt
		if sp.Tlength(n) > s3rdr.sz {
			n = uint64(s3rdr.sz)
		}
		region = "bytes=" + strconv.FormatUint(off, 10) + "-" + strconv.FormatUint(n-1, 10)
	}
	input := &s3.GetObjectInput{
		Bucket: &s3rdr.bucket,
		Key:    &key,
		Range:  &region,
	}

	result, err := s3rdr.clnt.GetObject(context.TODO(), input)
	if err != nil {
		return nil, 0, serr.NewErrError(err)
	}
	region1 := ""
	if result.ContentRange != nil {
		region1 = *result.ContentRange
	}
	db.DPrintf(db.S3, "s3Read: %v region %v res %v %v\n", s3rdr.key, region, region1, result.ContentLength)
	return result.Body, sp.Tlength(*result.ContentLength), nil
}

func (s3rdr *s3Reader) Read(off sp.Toffset, b []byte) (int, error) {
	db.DFatalf("Read: not implemented")
	return 0, nil
}

func (s3rdr *s3Reader) Lseek(off sp.Toffset) error {
	s3rdr.offset = off
	return nil
}

func (s3rdr *s3Reader) Close() error {
	return nil
}

func (s3rdr *s3Reader) GetReader() io.Reader {
	return s3rdr.rdr
}

func (s3rdr *s3Reader) Nbytes() sp.Tlength {
	return s3rdr.n
}

type rdr struct {
	s3rdr *s3Reader
	chunk io.ReadCloser
}

func (rdr *rdr) readChunk() error {
	r, n, err := rdr.s3rdr.s3Read(uint64(rdr.s3rdr.offset), CHUNKSZ)
	if err != nil {
		return err
	}
	rdr.chunk = r
	rdr.s3rdr.n += sp.Tlength(n)
	rdr.s3rdr.offset += sp.Toffset(n)
	return nil
}

func (rdr *rdr) Read(b []byte) (int, error) {
	db.DPrintf(db.S3, "s3.Read off %v sz %v len %d", rdr.s3rdr.offset, rdr.s3rdr.sz, len(b))
	if rdr.chunk == nil {
		if err := rdr.readChunk(); err != nil {
			db.DPrintf(db.S3, "readChunk err %v", err)
			return 0, err
		}
	}
	n, err := rdr.chunk.Read(b)
	if err == io.EOF && rdr.s3rdr.offset != sp.Toffset(rdr.s3rdr.sz) {
		rdr.chunk.Close()
		if err := rdr.readChunk(); err != nil {
			db.DPrintf(db.S3, "readChunk err %v", err)
			return n, err
		}
		return n, nil
	}
	db.DPrintf(db.S3, "s3.Read results %d", len(b))
	return n, err
}

func (rdr *rdr) Close() error {
	return rdr.chunk.Close()
}

func (fl *FsLib) getS3Client() *serr.Err {
	var ok bool
	s3secrets, ok := fl.pe.GetSecrets()["s3"]
	if !ok {
		return serr.NewErr(serr.TErrPerm, fmt.Errorf("Principal %v has no S3 secrets"))
	}
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
				return fl.GetNetProxyClnt().Dial(sp.NewEndpoint(sp.EXTERNAL_EP, []*sp.Taddr{a}))
			}
		})
		o.HTTPClient = hclnt
		o.UsePathStyle = true
	})
	fl.s3clnt = clnt
	return nil
}

func (fl *FsLib) OpenS3Reader(pn string) (ReaderSeekerI, error) {
	pn0, _ := strings.CutPrefix(pn, sp.S3+"~local/")
	p := path.Split(pn0)

	bucket := p[0]
	key := p[1:].String()

	db.DPrintf(db.S3, "OpenS3Reader %v(%v): bucket %q key %q", pn, pn0, bucket, key)

	if fl.s3clnt == nil {
		if err := fl.getS3Client(); err != nil {
			return nil, err
		}
	}

	st, err := fl.Stat(pn)
	if err != nil {
		return nil, err
	}

	db.DPrintf(db.S3, "OpenS3Reader: Stat %v", st)

	reader := &s3Reader{
		clnt:      fl.s3clnt,
		bucket:    bucket,
		key:       key,
		offset:    0,
		chunkSize: 8 * 1024 * 1024, // 8 Mb chunk size
		sz:        st.Tlength(),
	}
	rdr := &rdr{s3rdr: reader}
	reader.rdr = rdr
	return reader, err
}
