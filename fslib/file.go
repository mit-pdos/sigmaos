package fslib

import (
	"bufio"
	"fmt"
	"io"
	// "strings"

	//	"time"

	"github.com/klauspost/readahead"

	"sigmaos/awriter"
	db "sigmaos/debug"
	"sigmaos/reader"
	sos "sigmaos/sigmaos"
	sp "sigmaos/sigmap"
	"sigmaos/writer"
)

//
// Wrappers
//

func (fsl *FsLib) Open(path string, m sp.Tmode) (int, error) {
	return fsl.FileAPI.Open(path, m, sos.O_NOW)
}

func (fsl *FsLib) OpenWait(path string, m sp.Tmode) (int, error) {
	return fsl.FileAPI.Open(path, m, sos.O_WAIT)
}

func (fl *FsLib) SetFile(fname string, data []byte, m sp.Tmode, off sp.Toffset) (sp.Tsize, error) {
	return fl.FileAPI.PutFile(fname, 0777, m, data, off, sp.NoLeaseId)
}

func (fl *FsLib) PutFile(fname string, perm sp.Tperm, mode sp.Tmode, data []byte) (sp.Tsize, error) {
	return fl.FileAPI.PutFile(fname, perm, mode, data, 0, sp.NoLeaseId)
}

func (fl *FsLib) PutLeasedFile(fname string, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, data []byte) (sp.Tsize, error) {
	return fl.FileAPI.PutFile(fname, perm, mode, data, 0, lid)
}

//
// Open readers
//

type ReaderSeekerI interface {
	reader.ReaderI
	Lseek(off sp.Toffset) error
	GetReader() io.Reader
	Nbytes() sp.Tlength
}

type FdReader struct {
	*reader.Reader
	sof sos.FileAPI
	fd  int
}

func (rd *FdReader) GetReader() io.Reader {
	return rd.Reader
}

func (rd *FdReader) Close() error {
	return rd.sof.CloseFd(rd.fd)
}

func (rd *FdReader) Read(o sp.Toffset, b []byte) (int, error) {
	sz, err := rd.sof.Read(rd.fd, b)
	return int(sz), err
}

func (rd *FdReader) Fd() int {
	return rd.fd
}

func (rd *FdReader) Nbytes() sp.Tlength {
	return rd.Reader.Nbytes()
}

func (rd *FdReader) Lseek(off sp.Toffset) error {
	return rd.sof.Seek(rd.fd, off)
}

func newFdReader(sos sos.FileAPI, fd int) *FdReader {
	return &FdReader{nil, sos, fd}
}

func (fl *FsLib) NewReader(fd int, path string) *FdReader {
	fdrdr := newFdReader(fl.FileAPI, fd)
	fdrdr.Reader = reader.NewReader(fdrdr, path)
	return fdrdr
}

func (fl *FsLib) NewWriter(fd int) *writer.Writer {
	return writer.NewWriter(fl.FileAPI, fd)
}

func (fl *FsLib) OpenReader(path string) (ReaderSeekerI, error) {
	//if strings.Contains(path, sp.S3) {
	//return fl.OpenS3Reader(path)
	//}
	fd, err := fl.Open(path, sp.OREAD)
	if err != nil {
		return nil, err
	}
	return fl.NewReader(fd, path), nil
}

type Rdr struct {
	rdr ReaderSeekerI
	//brdr *bufio.Reader
	ardr io.ReadCloser
}

func (rdr *Rdr) Close() error {
	if err := rdr.ardr.Close(); err != nil {
		return err
	}
	if err := rdr.rdr.Close(); err != nil {
		return err
	}
	return nil
}

func (rdr *Rdr) Read(p []byte) (n int, err error) {
	return rdr.ardr.Read(p)
}

func (rdr *Rdr) Nbytes() sp.Tlength {
	return rdr.rdr.Nbytes()
}

func (fl *FsLib) OpenAsyncReader(path string, offset sp.Toffset) (*Rdr, error) {
	rdr, err := fl.OpenReader(path)
	if err != nil {
		return nil, err
	}
	r := &Rdr{rdr: rdr}
	if err := rdr.Lseek(offset); err != nil {
		return nil, err
	}
	// r.brdr = bufio.NewReaderSize(rdr.GetReader(), sp.BUFSZ)
	r.ardr, err = readahead.NewReaderSize(rdr.GetReader(), 4, sp.BUFSZ)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// func (fl *FsLib) OpenS3AsyncReader(path string, offset sp.Toffset) (io.ReadCloser, error) {
// 	rdr, err := fl.OpenS3Reader(path)
// 	if err != nil {
// 		return nil, err
// 	}
// 	r := &Rdr{rdr: rdr}
// 	if err := rdr.Lseek(offset); err != nil {
// 		return nil, err
// 	}
// 	// r.brdr = bufio.NewReaderSize(rdr.GetReader(), sp.BUFSZ)
// 	r.ardr, err = readahead.NewReaderSize(rdr.GetReader(), 4, sp.BUFSZ)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return r, nil
// }

func (fl *FsLib) OpenWaitReader(path string) (*FdReader, error) {
	fd, err := fl.FileAPI.Open(path, sp.OREAD, sos.O_WAIT)
	db.DPrintf(db.FSLIB, "OpenWaitReader %v err %v\n", path, err)
	if err != nil {
		return nil, err
	}
	rdr := fl.NewReader(fd, path)
	return rdr, nil

}

func (fl *FsLib) GetFileWatch(path string) ([]byte, error) {
	rdr, err := fl.OpenWaitReader(path)
	if err != nil {
		return nil, err
	}
	defer rdr.Close()
	b, error := rdr.GetData()
	if error != nil {
		return nil, error
	}
	return b, nil
}

//
// Writers
//

type WriterI interface {
	io.WriteCloser
	Nbytes() sp.Tlength
}

func (fl *FsLib) CreateWriter(fname string, perm sp.Tperm, mode sp.Tmode) (*writer.Writer, error) {
	fd, err := fl.Create(fname, perm, mode)
	if err != nil {
		return nil, err
	}
	wrt := fl.NewWriter(fd)
	return wrt, nil
}

func (fl *FsLib) OpenWriter(fname string, mode sp.Tmode) (*writer.Writer, error) {
	fd, err := fl.Open(fname, mode)
	if err != nil {
		return nil, err
	}
	wrt := fl.NewWriter(fd)
	return wrt, nil
}

type Wrt struct {
	wrt  WriterI
	awrt *awriter.Writer
	bwrt *bufio.Writer
}

func (fl *FsLib) CreateAsyncWriter(fname string, perm sp.Tperm, mode sp.Tmode) (WriterI, error) {
	w, err := fl.CreateWriter(fname, perm, mode)
	if err != nil {
		return nil, err
	}
	aw := awriter.NewWriterSize(w, 4, sp.BUFSZ)
	bw := bufio.NewWriterSize(aw, sp.BUFSZ)
	return &Wrt{w, aw, bw}, nil
}

func (fl *FsLib) CreateS3AsyncWriter(fname string, perm sp.Tperm, mode sp.Tmode) (WriterI, error) {
	w, err := fl.OpenS3Writer(fname)
	if err != nil {
		return nil, err
	}
	// aw := awriter.NewWriterSize(w, 4, sp.BUFSZ)
	bw := bufio.NewWriterSize(w, sp.BUFSZ)
	return &Wrt{w, nil, bw}, nil
}

func (wrt *Wrt) Close() error {
	if err := wrt.bwrt.Flush(); err != nil {
		return err
	}
	if wrt.awrt != nil {
		if err := wrt.awrt.Close(); err != nil {
			return err
		}
	}
	if err := wrt.wrt.Close(); err != nil {
		return err
	}
	return nil
}

func (wrt *Wrt) Write(b []byte) (int, error) {
	return wrt.bwrt.Write(b)
}

func (wrt *Wrt) Nbytes() sp.Tlength {
	return wrt.wrt.Nbytes()
}

//
// Util
//

// XXX use reader/writer interfaces
func (fl *FsLib) CopyFile(src, dst string) error {
	//	start := time.Now()
	//	defer func(t *time.Time) {
	//		db.DPrintf(db.ALWAYS, "Time reading + writing in copyFile: %v", time.Since(*t))
	//	}(&start)
	rdr, err := fl.OpenAsyncReader(src, 0)
	if err != nil {
		return err
	}
	//	db.DPrintf(db.ALWAYS, "Time openReader: %v", time.Since(start))
	//	start = time.Now()
	defer rdr.Close()
	wrt, err := fl.CreateAsyncWriter(dst, 0777, sp.OWRITE)
	if err != nil {
		return err
	}
	//	db.DPrintf(db.ALWAYS, "Time openWriter: %v", time.Since(start))
	defer wrt.Close()
	b := make([]byte, sp.BUFSZ)
	// Set start to ignore opening & closing reader/writer
	//	start = time.Now()
	for {
		//		start := time.Now()
		n, err := rdr.Read(b)
		if err != nil && err != io.EOF {
			return err
		}
		// Nothing left to read
		if n == 0 {
			break
		}
		//		db.DPrintf(db.ALWAYS, "Time reading in copyFile: %v", time.Since(start))
		b2 := b[:n]
		nn, err := wrt.Write(b2)
		if err != nil {
			return err
		}
		if nn != n {
			return fmt.Errorf("short write %v != %v", nn, n)
		}
	}
	return nil
}
