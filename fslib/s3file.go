package fslib

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
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

func (s3r *s3Reader) s3Read(off, cnt uint64) (io.ReadCloser, sp.Tlength, error) {
	db.DPrintf(db.S3, "s3Read %d %d", off, cnt)
	key := s3r.key
	region := ""
	if off != 0 || sp.Tlength(cnt) < s3r.sz {
		n := off + cnt
		if sp.Tlength(n) > s3r.sz {
			n = uint64(s3r.sz)
		}
		region = "bytes=" + strconv.FormatUint(off, 10) + "-" + strconv.FormatUint(n-1, 10)
	}
	input := &s3.GetObjectInput{
		Bucket: &s3r.bucket,
		Key:    &key,
		Range:  &region,
	}

	result, err := s3r.clnt.GetObject(context.TODO(), input)
	if err != nil {
		return nil, 0, serr.NewErrError(err)
	}
	region1 := ""
	if result.ContentRange != nil {
		region1 = *result.ContentRange
	}
	db.DPrintf(db.S3, "s3Read: %v region %v res %v %v\n", s3r.key, region, region1, result.ContentLength)
	return result.Body, sp.Tlength(*result.ContentLength), nil
}

func (s3r *s3Reader) Read(off sp.Toffset, b []byte) (int, error) {
	db.DFatalf("Read: not implemented")
	return 0, nil
}

func (s3r *s3Reader) Lseek(off sp.Toffset) error {
	s3r.offset = off
	return nil
}

func (s3r *s3Reader) Close() error {
	return nil
}

func (s3r *s3Reader) GetReader() io.Reader {
	return s3r.rdr
}

func (s3r *s3Reader) Nbytes() sp.Tlength {
	return s3r.n
}

type rdr struct {
	s3r   *s3Reader
	chunk io.ReadCloser
}

func (rdr *rdr) readChunk() error {
	r, n, err := rdr.s3r.s3Read(uint64(rdr.s3r.offset), CHUNKSZ)
	if err != nil {
		return err
	}
	rdr.chunk = r
	rdr.s3r.n += sp.Tlength(n)
	rdr.s3r.offset += sp.Toffset(n)
	return nil
}

func (rdr *rdr) Read(b []byte) (int, error) {
	db.DPrintf(db.S3, "s3.Read off %v sz %v len %d", rdr.s3r.offset, rdr.s3r.sz, len(b))
	if rdr.chunk == nil {
		if err := rdr.readChunk(); err != nil {
			db.DPrintf(db.S3, "readChunk err %v", err)
			return 0, err
		}
	}
	n, err := rdr.chunk.Read(b)
	if err == io.EOF && rdr.s3r.offset != sp.Toffset(rdr.s3r.sz) {
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
		return serr.NewErr(serr.TErrPerm, fmt.Errorf("Principal has no S3 secrets"))
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

func (fl *FsLib) S3Stat(bucket, key string) (sp.Tlength, error) {
	if fl.s3clnt == nil {
		if err := fl.getS3Client(); err != nil {
			return 0, err
		}
	}

	input := &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	result, err := fl.s3clnt.HeadObject(context.TODO(), input)
	if err != nil {
		db.DPrintf(db.S3, "readHead: %v err %v\n", key, err)
		return 0, serr.NewErrError(err)
	}
	db.DPrintf(db.S3, "readHead: %v %v %v\n", key, result.ContentLength, err)
	return sp.Tlength(*result.ContentLength), nil
}

func (fl *FsLib) OpenS3Reader(pn string) (ReaderSeekerI, error) {
	pn0, _ := strings.CutPrefix(pn, sp.S3+"~local/")
	p := path.Split(pn0)

	bucket := p[0]
	key := p[1:].String()

	db.DPrintf(db.S3, "OpenS3Reader %v: bucket %q key %q", pn, bucket, key)

	if fl.s3clnt == nil {
		if err := fl.getS3Client(); err != nil {
			return nil, err
		}
	}

	sz, err := fl.S3Stat(bucket, key)
	if err != nil {
		return nil, err
	}

	db.DPrintf(db.S3, "OpenS3Reader: S3Stat %v", sz)

	reader := &s3Reader{
		clnt:      fl.s3clnt,
		bucket:    bucket,
		key:       key,
		offset:    0,
		chunkSize: 6 * MB,
		sz:        sz,
	}
	rdr := &rdr{s3r: reader}
	reader.rdr = rdr
	return reader, err
}

type s3Writer struct {
	clnt      *s3.Client
	rdr       io.WriteCloser
	bucket    string
	key       string
	offset    sp.Toffset
	chunkSize int64
	sz        sp.Tlength
	n         sp.Tlength
	r         *io.PipeReader
	w         *io.PipeWriter
	ch        chan error
}

func (s3w *s3Writer) writer() {
	s3w.r, s3w.w = io.Pipe()
	uploader := manager.NewUploader(s3w.clnt)
	_, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: &s3w.bucket,
		Key:    &s3w.key,
		Body:   s3w.r,
	})
	if err != nil {
		db.DPrintf(db.S3, "Writer %v err %v\n", s3w.key, err)
	}
	s3w.ch <- err
}

func (s3w *s3Writer) Write(b []byte) (int, error) {
	db.DPrintf(db.S3, "Write %v off %v f %v\n", len(b), s3w.offset, s3w.key)
	if n, err := s3w.w.Write(b); err != nil {
		db.DPrintf(db.S3, "Write %v %v err %v\n", s3w.offset, len(b), err)
		return 0, serr.NewErrError(err)
	} else {
		s3w.offset += sp.Toffset(n)
		s3w.sz = sp.Tlength(s3w.offset)
		return n, nil
	}
}

func (s3w *s3Writer) Close() error {
	s3w.w.Close()
	// wait for uploader to finish
	err := <-s3w.ch
	if err != nil {
		return serr.NewErrError(err)
	}
	return nil
}

func (s3w *s3Writer) Nbytes() sp.Tlength {
	return s3w.sz
}

func (fl *FsLib) OpenS3Writer(pn string) (WriterI, error) {
	pn0, _ := strings.CutPrefix(pn, sp.S3+"~local/")
	p := path.Split(pn0)

	bucket := p[0]
	key := p[1:].String()

	db.DPrintf(db.S3, "OpenS3Writer %v: bucket %q key %q", pn, bucket, key)

	if fl.s3clnt == nil {
		if err := fl.getS3Client(); err != nil {
			return nil, err
		}
	}

	writer := &s3Writer{
		clnt:      fl.s3clnt,
		bucket:    bucket,
		key:       key,
		chunkSize: 5 * MB,
		ch:        make(chan error),
	}
	go writer.writer()
	return writer, nil
}
