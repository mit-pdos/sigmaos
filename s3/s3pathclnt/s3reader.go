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
	clnt   *s3.Client
	bucket string
	key    string
	sz     sp.Tlength
	n      sp.Tlength
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
	// db.DPrintf(db.S3CLNT, "s3Read: %v %d %d res %v %v\n", s3r.key, off, cnt, region, result.ContentLength)
	return result.Body, sp.Tlength(*result.ContentLength), nil
}

func (s3r *s3Reader) readChunk(off sp.Toffset, len int) (io.ReadCloser, error) {
	if sp.Tlength(off) >= s3r.sz {
		return nil, io.EOF
	}
	r, _, err := s3r.s3Read(uint64(off), uint64(len))
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s3r *s3Reader) read(off sp.Toffset, b []byte) (int, error) {
	// db.DPrintf(db.S3CLNT, "s3.Read off %d len %d", off, len(b))
	if off >= sp.Toffset(s3r.sz) {
		return 0, io.EOF
	}
	if chunk, err := s3r.readChunk(off, len(b)); err != nil {
		db.DPrintf(db.S3CLNT, "readChunk err %v", err)
		return 0, err
	} else {
		i := 0
		l := len(b)
		for i < l {
			n, err := chunk.Read(b[i:l])
			if err != nil && err != io.EOF {
				return 0, err
			}
			i += n
		}
		db.DPrintf(db.S3CLNT, "s3.Read off %d end %d buflen %d n %d err %v", off, s3r.sz, len(b), i, err)
		chunk.Close()
		return i, nil
	}
}

func (s3r *s3Reader) readRdr(off sp.Toffset, sz sp.Tsize) (io.ReadCloser, error) {
	db.DPrintf(db.S3CLNT, "s3.ReadRdr off %d len %d", off, sz)
	if off >= sp.Toffset(s3r.sz) {
		return nil, io.EOF
	}
	if chunk, err := s3r.readChunk(off, int(sz)); err != nil {
		db.DPrintf(db.S3CLNT, "readChunk err %v", err)
		return nil, err
	} else {
		return chunk, nil
	}
}

func (s3r *s3Reader) close() error {
	return nil
}
