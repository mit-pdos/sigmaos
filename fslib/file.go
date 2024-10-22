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
	"sigmaos/fslib/reader"
	"sigmaos/fslib/writer"
	sos "sigmaos/sigmaos"
	sp "sigmaos/sigmap"
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

// For clients of fslib that want an io.Reader interface for a file with
// a few extra features (e.g., reading no more than len bytes, if len > 0).
type FileReader struct {
	rdr *reader.Reader
	fd  int
	len sp.Tlength
	n   sp.Tlength
	pn  string
}

func newFileReader(rdr *reader.Reader, fd int, len sp.Tlength, pn string) *FileReader {
	return &FileReader{rdr, fd, len, 0, pn}
}

func (rd *FileReader) Close() error {
	return rd.rdr.Close()
}

func (rd *FileReader) Fd() int {
	return rd.fd
}

// Read no more than len bytes, if len is set
func (rd *FileReader) Read(b []byte) (int, error) {
	if rd.len != 0 && rd.n >= rd.len {
		return 0, io.EOF
	}
	sz, err := rd.rdr.Read(b)
	rd.n += sp.Tlength(sz)
	return sz, err
}

func (rd *FileReader) Nbytes() sp.Tlength {
	return rd.n
}

func (fl *FsLib) NewReaderRegion(fd int, path string, len sp.Tlength) *FileReader {
	fdrdr := newFdReader(fl.FileAPI, fd)
	rdr := reader.NewReader(fdrdr, path)
	return newFileReader(rdr, fd, len, path)
}

func (fl *FsLib) NewReader(fd int, path string) *FileReader {
	return fl.NewReaderRegion(fd, path, 0)
}

func (fl *FsLib) OpenReader(path string) (*FileReader, error) {
	fd, err := fl.Open(path, sp.OREAD)
	if err != nil {
		return nil, err
	}
	return fl.NewReader(fd, path), nil
}

func (fl *FsLib) OpenReaderRegion(path string, offset sp.Toffset, len sp.Tlength) (*FileReader, error) {
	fd, err := fl.Open(path, sp.OREAD)
	if err != nil {
		return nil, err
	}
	fl.Seek(fd, offset)
	return fl.NewReaderRegion(fd, path, len), nil
}

type BufFileReader struct {
	*FileReader
	brdr *bufio.Reader
}

func (rdr *BufFileReader) Close() error {
	if err := rdr.FileReader.Close(); err != nil {
		return err
	}
	return nil
}

func (rdr *BufFileReader) Read(p []byte) (n int, err error) {
	return rdr.brdr.Read(p)
}

func (fl *FsLib) OpenBufReader(path string) (*BufFileReader, error) {
	fd, err := fl.Open(path, sp.OREAD)
	if err != nil {
		return nil, err
	}
	rdr := fl.NewReader(fd, path)
	brdr := bufio.NewReaderSize(rdr, sp.BUFSZ)
	return &BufFileReader{rdr, brdr}, nil
}

type AsyncFileReader struct {
	*FileReader
	ardr io.ReadCloser
}

func (rdr *AsyncFileReader) Close() error {
	if rdr.ardr != nil {
		if err := rdr.ardr.Close(); err != nil {
			return err
		}
	}
	if err := rdr.rdr.Close(); err != nil {
		return err
	}
	return nil
}

func (rdr *AsyncFileReader) Read(p []byte) (n int, err error) {
	if rdr.ardr != nil {
		return rdr.ardr.Read(p)
	}
	return rdr.FileReader.Read(p)
}

func (fl *FsLib) OpenAsyncReader(path string, offset sp.Toffset) (*AsyncFileReader, error) {
	rdr, err := fl.OpenReaderRegion(path, offset, 0)
	if err != nil {
		return nil, err
	}
	r := &AsyncFileReader{FileReader: rdr}
	r.ardr, err = readahead.NewReaderSize(rdr, 4, sp.BUFSZ)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (fl *FsLib) OpenWaitReader(path string) (int, error) {
	fd, err := fl.FileAPI.Open(path, sp.OREAD, sos.O_WAIT)
	db.DPrintf(db.FSLIB, "OpenWaitReader %v err %v\n", path, err)
	if err != nil {
		return 0, err
	}
	return fd, nil
}

func (fl *FsLib) GetFileWatch(path string) ([]byte, error) {
	fd, err := fl.OpenWaitReader(path)
	if err != nil {
		return nil, err
	}
	defer fl.FileAPI.CloseFd(fd)
	b := make([]byte, sp.MAXGETSET)
	sz, error := fl.FileAPI.Read(fd, b)
	if error != nil {
		return nil, error
	}
	return b[:sz], nil
}

// File readers pass fdReader to reader to provide an io.Reader
// interface
type fdReader struct {
	sof sos.FileAPI
	fd  int
}

func newFdReader(sos sos.FileAPI, fd int) *fdReader {
	return &fdReader{sos, fd}
}

func (rd *fdReader) Close() error {
	return rd.sof.CloseFd(rd.fd)
}

func (rd *fdReader) Read(off sp.Toffset, b []byte) (int, error) {
	sz, err := rd.sof.Read(rd.fd, b)
	return int(sz), err
}

//
// Writers
//

type FileWriter struct {
	wrt  *writer.Writer
	awrt *awriter.Writer
	bwrt *bufio.Writer
}

func (fl *FsLib) newFileWriter(fd int) *FileWriter {
	w := writer.NewWriter(fl.FileAPI, fd)
	return &FileWriter{w, nil, nil}
}

func (fl *FsLib) newBufFileWriter(fd int) *FileWriter {
	w := writer.NewWriter(fl.FileAPI, fd)
	bw := bufio.NewWriterSize(w, sp.BUFSZ)
	return &FileWriter{w, nil, bw}
}

func (fl *FsLib) newBufAsyncFileWriter(fd int) *FileWriter {
	w := writer.NewWriter(fl.FileAPI, fd)
	aw := awriter.NewWriterSize(w, 4, sp.BUFSZ)
	bw := bufio.NewWriterSize(aw, sp.BUFSZ)
	return &FileWriter{w, aw, bw}
}

func (wrt *FileWriter) Close() error {
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

func (wrt *FileWriter) Write(b []byte) (int, error) {
	if wrt.bwrt != nil {
		return wrt.bwrt.Write(b)
	}
	return wrt.wrt.Write(b)
}

func (wrt *FileWriter) Nbytes() sp.Tlength {
	return wrt.wrt.Nbytes()
}

func (fl *FsLib) CreateWriter(fname string, perm sp.Tperm, mode sp.Tmode) (*FileWriter, error) {
	fd, err := fl.Create(fname, perm, mode)
	if err != nil {
		return nil, err
	}
	return fl.newFileWriter(fd), nil
}

func (fl *FsLib) OpenWriter(fname string) (*FileWriter, error) {
	fd, err := fl.Open(fname, sp.OWRITE)
	if err != nil {
		return nil, err
	}
	return fl.newFileWriter(fd), nil
}

func (fl *FsLib) CreateBufWriter(fname string, perm sp.Tperm) (*FileWriter, error) {
	fd, err := fl.Create(fname, perm, sp.OWRITE)
	if err != nil {
		return nil, err
	}
	return fl.newBufFileWriter(fd), nil
}

func (fl *FsLib) OpenBufWriter(fname string, mode sp.Tmode) (*FileWriter, error) {
	fd, err := fl.Open(fname, mode)
	if err != nil {
		return nil, err
	}
	return fl.newBufFileWriter(fd), nil
}

func (fl *FsLib) CreateAsyncWriter(fname string, perm sp.Tperm, mode sp.Tmode) (*FileWriter, error) {
	fd, err := fl.Create(fname, perm, mode)
	if err != nil {
		return nil, err
	}
	return fl.newBufAsyncFileWriter(fd), nil
}

//
// Util
//

func (fl *FsLib) CopyFile(src, dst string) error {
	//	start := time.Now()
	//	defer func(t *time.Time) {
	//		db.DPrintf(db.ALWAYS, "Time reading + writing in copyFile: %v", time.Since(*t))
	//	}(&start)
	rdr, err := fl.OpenReader(src)
	if err != nil {
		return err
	}
	//	db.DPrintf(db.ALWAYS, "Time openReader: %v", time.Since(start))
	//	start = time.Now()
	defer rdr.Close()
	wrt, err := fl.CreateWriter(dst, 0777, sp.OWRITE)
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
