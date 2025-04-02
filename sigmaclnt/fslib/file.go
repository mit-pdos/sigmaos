package fslib

import (
	"bufio"
	"fmt"
	"io"
	"sync"

	//	"time"

	sos "sigmaos/api/sigmaos"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt/fslib/reader"
	"sigmaos/sigmaclnt/fslib/writer"
	sp "sigmaos/sigmap"
)

//
// Wrappers
//

func (fsl *FsLib) Open(pn sp.Tsigmapath, m sp.Tmode) (int, error) {
	return fsl.FileAPI.Open(pn, m, sos.O_NOW)
}

func (fsl *FsLib) OpenWait(pn sp.Tsigmapath, m sp.Tmode) (int, error) {
	return fsl.FileAPI.Open(pn, m, sos.O_WAIT)
}

func (fl *FsLib) SetFile(pn sp.Tsigmapath, data []byte, m sp.Tmode, off sp.Toffset) (sp.Tsize, error) {
	return fl.FileAPI.PutFile(pn, 0777, m, data, off, sp.NoLeaseId)
}

func (fl *FsLib) PutFile(pn sp.Tsigmapath, perm sp.Tperm, mode sp.Tmode, data []byte) (sp.Tsize, error) {
	return fl.FileAPI.PutFile(pn, perm, mode, data, 0, sp.NoLeaseId)
}

func (fl *FsLib) PutLeasedFile(pn sp.Tsigmapath, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, data []byte) (sp.Tsize, error) {
	return fl.FileAPI.PutFile(pn, perm, mode, data, 0, lid)
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
	pn  sp.Tsigmapath
}

func newFileReader(rdr *reader.Reader, fd int, len sp.Tlength, pn sp.Tsigmapath) *FileReader {
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

func (fl *FsLib) NewReaderRegion(fd int, pn sp.Tsigmapath, len sp.Tlength) *FileReader {
	fdrdr := newFdReader(fl.FileAPI, fd)
	rdr := reader.NewReader(fdrdr, pn)
	return newFileReader(rdr, fd, len, pn)
}

func (fl *FsLib) NewReader(fd int, pn sp.Tsigmapath) *FileReader {
	return fl.NewReaderRegion(fd, pn, 0)
}

func (fl *FsLib) OpenReader(pn sp.Tsigmapath) (*FileReader, error) {
	fd, err := fl.Open(pn, sp.OREAD)
	if err != nil {
		return nil, err
	}
	return fl.NewReader(fd, pn), nil
}

func (fl *FsLib) OpenReaderRegion(pn sp.Tsigmapath, offset sp.Toffset, len sp.Tlength) (*FileReader, error) {
	fd, err := fl.Open(pn, sp.OREAD)
	if err != nil {
		return nil, err
	}
	fl.Seek(fd, offset)
	return fl.NewReaderRegion(fd, pn, len), nil
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

func (fl *FsLib) OpenBufReader(pn sp.Tsigmapath) (*BufFileReader, error) {
	fd, err := fl.Open(pn, sp.OREAD)
	if err != nil {
		return nil, err
	}
	rdr := fl.NewReader(fd, pn)
	brdr := bufio.NewReaderSize(rdr, sp.BUFSZ)
	return &BufFileReader{rdr, brdr}, nil
}

type ParallelFileReader struct {
	fd  int
	sof sos.FileAPI
	end sp.Toffset

	mu  sync.Mutex
	err error
	off sp.Toffset // next offset to consume
}

func (fl *FsLib) OpenParallelFileReader(pn sp.Tsigmapath, offset sp.Toffset, l sp.Tlength) (*ParallelFileReader, error) {
	fd, err := fl.Open(pn, sp.OREAD)
	if err != nil {
		return nil, err
	}
	r := &ParallelFileReader{
		fd:  fd,
		sof: fl.FileAPI,
		end: offset + sp.Toffset(l),
		off: offset,
	}
	return r, nil
}

// caller can use offinc to arrange for some overlap between two chunks
func (pfr *ParallelFileReader) getChunk(sz, offinc int) (sp.Toffset, sp.Toffset, error) {
	pfr.mu.Lock()
	defer pfr.mu.Unlock()

	if pfr.off >= pfr.end {
		return pfr.end, pfr.end, io.EOF
	}

	off := pfr.off
	e := off + sp.Toffset(sz)
	if pfr.end < e {
		e = pfr.end
	}
	pfr.off += sp.Toffset(offinc)
	return off, e, nil
}

func (pfr *ParallelFileReader) GetChunkReader(sz, offinc int) (io.ReadCloser, sp.Toffset, error) {
	o, e, err := pfr.getChunk(sz, offinc)
	if err != nil {
		return nil, 0, err
	}
	db.DPrintf(db.PREADER, "GetChunkReader: %v %v", o, e)
	r, err := pfr.sof.PreadRdr(pfr.fd, o, sp.Tsize(e-o))
	return r, o, err
}

func (pfr *ParallelFileReader) Close() error {
	return pfr.sof.CloseFd(pfr.fd)
}

func (fl *FsLib) OpenWaitReader(pn sp.Tsigmapath) (int, error) {
	fd, err := fl.FileAPI.Open(pn, sp.OREAD, sos.O_WAIT)
	db.DPrintf(db.FSLIB, "OpenWaitReader %v err %v\n", pn, err)
	if err != nil {
		return 0, err
	}
	return fd, nil
}

func (fl *FsLib) GetFileWatch(pn sp.Tsigmapath) ([]byte, error) {
	fd, err := fl.OpenWaitReader(pn)
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
	bwrt *bufio.Writer
}

func (fl *FsLib) newFileWriter(fd int) *FileWriter {
	w := writer.NewWriter(fl.FileAPI, fd)
	return &FileWriter{w, nil}
}

func (fl *FsLib) newBufFileWriter(fd int) *FileWriter {
	w := writer.NewWriter(fl.FileAPI, fd)
	bw := bufio.NewWriterSize(w, sp.BUFSZ)
	return &FileWriter{w, bw}
}

func (wrt *FileWriter) Close() error {
	if wrt.bwrt != nil {
		if err := wrt.bwrt.Flush(); err != nil {
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

func (fl *FsLib) CreateWriter(pn sp.Tsigmapath, perm sp.Tperm, mode sp.Tmode) (*FileWriter, error) {
	fd, err := fl.Create(pn, perm, mode)
	if err != nil {
		return nil, err
	}
	return fl.newFileWriter(fd), nil
}

func (fl *FsLib) OpenWriter(pn sp.Tsigmapath) (*FileWriter, error) {
	fd, err := fl.Open(pn, sp.OWRITE)
	if err != nil {
		return nil, err
	}
	return fl.newFileWriter(fd), nil
}

func (fl *FsLib) CreateBufWriter(pn sp.Tsigmapath, perm sp.Tperm) (*FileWriter, error) {
	fd, err := fl.Create(pn, perm, sp.OWRITE)
	if err != nil {
		return nil, err
	}
	return fl.newBufFileWriter(fd), nil
}

func (fl *FsLib) OpenBufWriter(pn sp.Tsigmapath, mode sp.Tmode) (*FileWriter, error) {
	fd, err := fl.Open(pn, mode)
	if err != nil {
		return nil, err
	}
	return fl.newBufFileWriter(fd), nil
}

//
// Util
//

func (fl *FsLib) CopyFile(src, dst sp.Tsigmapath) error {
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
