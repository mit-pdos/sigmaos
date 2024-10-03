package s3pathclnt

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "sigmaos/debug"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type s3Writer struct {
	clnt   *s3.Client
	rdr    io.WriteCloser
	bucket string
	key    string
	offset sp.Toffset
	sz     sp.Tlength
	n      sp.Tlength
	r      *io.PipeReader
	w      *io.PipeWriter
	ch     chan error
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
		db.DPrintf(db.S3CLNT, "Writer %v err %v\n", s3w.key, err)
	}
	s3w.ch <- err
}

func (s3w *s3Writer) Write(b []byte) (int, error) {
	db.DPrintf(db.S3CLNT, "Write %v off %v f %v\n", len(b), s3w.offset, s3w.key)
	if n, err := s3w.w.Write(b); err != nil {
		db.DPrintf(db.S3CLNT, "Write %v %v err %v\n", s3w.offset, len(b), err)
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
