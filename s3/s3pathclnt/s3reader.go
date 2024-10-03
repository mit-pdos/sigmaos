package s3pathclnt

import (
	"context"
	"io"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "sigmaos/debug"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type s3Reader struct {
	clnt    *s3.Client
	bucket  string
	key     string
	offset  sp.Toffset
	chunk   io.ReadCloser
	chunksz sp.Tlength
	sz      sp.Tlength
	n       sp.Tlength
}

func min64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func (s3r *s3Reader) s3Read(off, cnt uint64) (io.ReadCloser, sp.Tlength, *serr.Err) {
	key := s3r.key
	n := min64(cnt, uint64(s3r.sz)-off)
	region := "bytes=" + strconv.FormatUint(off, 10) + "-" + strconv.FormatUint(off+n-1, 10)
	input := &s3.GetObjectInput{
		Bucket: &s3r.bucket,
		Key:    &key,
		Range:  &region,
	}

	result, err := s3r.clnt.GetObject(context.TODO(), input)
	if err != nil {
		return nil, 0, serr.NewErrError(err)
	}
	db.DPrintf(db.S3CLNT, "s3Read: %v %d %d res %v %v\n", s3r.key, off, cnt, region, result.ContentLength)
	return result.Body, sp.Tlength(*result.ContentLength), nil
}

func (s3r *s3Reader) readChunk(off sp.Toffset) error {
	if sp.Tlength(off) >= s3r.sz {
		return io.EOF
	}
	r, n, err := s3r.s3Read(uint64(off), CHUNKSZ)
	if err != nil {
		return err
	}
	s3r.chunk = r
	s3r.offset = off
	s3r.chunksz = n
	return nil
}

func (s3r *s3Reader) read(off sp.Toffset, b []byte) (int, error) {
	// db.DPrintf(db.S3CLNT, "s3.Read off %d len %d", off, len(b))
	if off >= sp.Toffset(s3r.sz) {
		return 0, io.EOF
	}
	if s3r.chunk == nil {
		if err := s3r.readChunk(off); err != nil {
			db.DPrintf(db.S3CLNT, "readChunk err %v", err)
			return 0, err
		}
	}
	n, err := s3r.chunk.Read(b)
	s3r.offset += sp.Toffset(n)
	s3r.n += sp.Tlength(n)
	if err == io.EOF {
		s3r.chunk.Close()
		s3r.chunk = nil
	}
	// db.DPrintf(db.S3CLNT, "s3.Read off %d end %d buflen %d n %d err %v", off, s3r.sz, len(b), n, err)
	return n, nil
}

func (s3r *s3Reader) Close() error {
	if s3r.chunk != nil {
		return s3r.chunk.Close()
	}
	return nil
}
