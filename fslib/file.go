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

type ReaderI interface {
	io.ReadCloser
	Nbytes() sp.Tlength
}

type FdReader struct {
	*reader.Reader
	sof sos.FileAPI
	fd  int
	len sp.Tlength
}

func (rd *FdReader) Close() error {
	return rd.sof.CloseFd(rd.fd)
}

// Read no more than len bytes, if len is set
func (rd *FdReader) Read(o sp.Toffset, b []byte) (int, error) {
	if rd.len != 0 && rd.Nbytes() >= rd.len {
		return 0, io.EOF
	}
	sz, err := rd.sof.Read(rd.fd, b)
	return int(sz), err
}

func (rd *FdReader) Fd() int {
	return rd.fd
}

func (rd *FdReader) Nbytes() sp.Tlength {
	return rd.Reader.Nbytes()
}

func newFdReader(sos sos.FileAPI, fd int, len sp.Tlength) *FdReader {
	return &FdReader{nil, sos, fd, len}
}

func (fl *FsLib) NewReaderRegion(fd int, path string, len sp.Tlength) *FdReader {
	fdrdr := newFdReader(fl.FileAPI, fd, len)
	fdrdr.Reader = reader.NewReader(fdrdr, path)
	return fdrdr
}

func (fl *FsLib) NewReader(fd int, path string) *FdReader {
	return fl.NewReaderRegion(fd, path, 0)
}

func (fl *FsLib) NewWriter(fd int) *writer.Writer {
	return writer.NewWriter(fl.FileAPI, fd)
}

func (fl *FsLib) OpenReader(path string) (*FdReader, error) {
	fd, err := fl.Open(path, sp.OREAD)
	if err != nil {
		return nil, err
	}
	return fl.NewReader(fd, path), nil
}

func (fl *FsLib) OpenReaderRegion(path string, offset sp.Toffset, len sp.Tlength) (*FdReader, error) {
	fd, err := fl.Open(path, sp.OREAD)
	if err != nil {
		return nil, err
	}
	fl.Seek(fd, offset)
	return fl.NewReaderRegion(fd, path, len), nil
}

type Rdr struct {
	*FdReader
	brdr *bufio.Reader
	ardr io.ReadCloser
}

func (rdr *Rdr) Close() error {
	if rdr.ardr != nil {
		if err := rdr.ardr.Close(); err != nil {
			return err
		}
	}
	if err := rdr.FdReader.Close(); err != nil {
		return err
	}
	return nil
}

func (rdr *Rdr) Read(p []byte) (n int, err error) {
	if rdr.ardr != nil {
		return rdr.ardr.Read(p)
	}
	return rdr.brdr.Read(p)
}

func (fl *FsLib) OpenBufReader(path string, offset sp.Toffset) (ReaderI, error) {
	rdr, err := fl.OpenReaderRegion(path, offset, 0)
	if err != nil {
		return nil, err
	}
	r := &Rdr{FdReader: rdr}
	r.brdr = bufio.NewReaderSize(rdr.Reader, sp.BUFSZ)
	return r, nil
}

func (fl *FsLib) OpenAsyncReader(path string, offset sp.Toffset) (ReaderI, error) {
	rdr, err := fl.OpenReaderRegion(path, offset, 0)
	if err != nil {
		return nil, err
	}
	r := &Rdr{FdReader: rdr}
	//r.brdr = bufio.NewReaderSize(rdr.Reader, sp.BUFSZ)
	r.ardr, err = readahead.NewReaderSize(rdr.Reader, 4, sp.BUFSZ)
	if err != nil {
		return nil, err
	}
	return r, nil
}

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

type Wrt struct {
	wrt  WriterI
	awrt *awriter.Writer
	bwrt *bufio.Writer
}

func (fl *FsLib) newWrt(fd int) *Wrt {
	w := fl.NewWriter(fd)
	return &Wrt{w, nil, nil}
}

func (fl *FsLib) newBufWrt(fd int) *Wrt {
	w := fl.NewWriter(fd)
	bw := bufio.NewWriterSize(w, sp.BUFSZ)
	return &Wrt{w, nil, bw}
}

func (fl *FsLib) CreateWriter(fname string, perm sp.Tperm, mode sp.Tmode) (WriterI, error) {
	fd, err := fl.Create(fname, perm, mode)
	if err != nil {
		return nil, err
	}
	return fl.newWrt(fd), nil
}

func (fl *FsLib) OpenWriter(fname string) (WriterI, error) {
	fd, err := fl.Open(fname, sp.OWRITE)
	if err != nil {
		return nil, err
	}
	return fl.newWrt(fd), nil
}

func (fl *FsLib) CreateBufWriter(fname string, perm sp.Tperm) (WriterI, error) {
	fd, err := fl.Create(fname, perm, sp.OWRITE)
	if err != nil {
		return nil, err
	}
	return fl.newBufWrt(fd), nil
}

func (fl *FsLib) OpenBufWriter(fname string, mode sp.Tmode) (WriterI, error) {
	fd, err := fl.Open(fname, mode)
	if err != nil {
		return nil, err
	}
	return fl.newBufWrt(fd), nil
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

func (wrt *Wrt) Close() error {
	if wrt.bwrt != nil {
		if err := wrt.bwrt.Flush(); err != nil {
			return err
		}
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
	if wrt.bwrt != nil {
		return wrt.bwrt.Write(b)
	}
	return wrt.wrt.Write(b)
}

func (wrt *Wrt) Nbytes() sp.Tlength {
	return wrt.wrt.Nbytes()
}

//
// Util
//

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
