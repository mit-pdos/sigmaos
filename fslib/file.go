package fslib

import (
	"bufio"
	"fmt"
	"io"
	"sync"
	//	"time"

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

type ParallelFileReader struct {
	fd  int
	sof sos.FileAPI
	end sp.Toffset

	mu  sync.Mutex
	err error
	off sp.Toffset // next offset to consume
}

func (fl *FsLib) OpenParallelFileReader(path string, offset sp.Toffset, l sp.Tlength) (*ParallelFileReader, error) {
	fd, err := fl.Open(path, sp.OREAD)
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

type AsyncFileReader struct {
	fd      int
	sof     sos.FileAPI
	end     sp.Toffset
	wg      sync.WaitGroup
	rdrs    []io.ReadCloser
	buf     []byte
	nreader int
	done    []bool

	mu       sync.Mutex
	pcond    *sync.Cond
	ccond    *sync.Cond
	err      error
	coff     sp.Toffset // next offset to consume
	poff     sp.Toffset // next offset to be produced
	noff     sp.Toffset // next offset to start reading
	isClosed bool
}

// Close() maybe called before the caller has read the complete
// region.
func (rdr *AsyncFileReader) Close() error {
	db.DPrintf(db.PREADER, "Close coff %d poff %d end %d", rdr.coff, rdr.poff, rdr.end)
	rdr.close()
	rdr.wg.Wait()
	return rdr.sof.CloseFd(rdr.fd)
}

func (rdr *AsyncFileReader) Read(p []byte) (int, error) {
	rdr.mu.Lock()
	defer rdr.mu.Unlock()

	if rdr.err != nil {
		return 0, rdr.err
	}
	if rdr.coff >= rdr.end {
		rdr.err = io.EOF
		return 0, io.EOF
	}
	for rdr.coff == rdr.poff {
		db.DPrintf(db.PREADER, "Read: wait for chunk p %d c %d", chunkId(rdr.poff), chunkId(rdr.coff))
		rdr.ccond.Wait()
	}

	if rdr.coff > rdr.poff {
		db.DFatalf("Read: c %d(%d) p %d(%d)", chunkId(rdr.coff), rdr.coff, chunkId(rdr.poff), rdr.poff)
	}
	i := chunkId(rdr.coff)
	n, err := rdr.rdrs[chunk2Buf(i, rdr.nreader)].Read(p)
	rdr.coff += sp.Toffset(n)

	db.DPrintf(db.PREADER, "Read n %d ck %d off %d(%d) end %d err %v", n, chunkId(rdr.coff), rdr.coff, chunkOff(rdr.coff), rdr.end, err)
	if rdr.coff >= rdr.poff {
		rdr.pcond.Broadcast()
	}
	return n, nil
}

func (rdr *AsyncFileReader) close() {
	rdr.mu.Lock()
	defer rdr.mu.Unlock()
	rdr.isClosed = true
	rdr.pcond.Broadcast()
}

func (rdr *AsyncFileReader) setErr(err error) {
	rdr.mu.Lock()
	defer rdr.mu.Unlock()
	rdr.err = err
}

func (rdr *AsyncFileReader) getChunk() (sp.Toffset, bool) {
	rdr.mu.Lock()
	defer rdr.mu.Unlock()

	db.DPrintf(db.PREADER, "getChunk: off %d end %d", rdr.noff, rdr.end)
	off := rdr.noff
	rdr.noff += sp.BUFSZ
	if off >= rdr.end || rdr.err != nil {
		return 0, false
	}
	return off, true
}

func chunkId(off sp.Toffset) int {
	return int(off / sp.BUFSZ)
}

func chunkOff(off sp.Toffset) int {
	return int(off % sp.BUFSZ)
}

func chunk2Offset(ck int) int {
	return ck * sp.BUFSZ
}

func chunk2Buf(i, n int) int {
	return i % n
}

func (rdr *AsyncFileReader) getBuf(off sp.Toffset) (int, bool) {
	rdr.mu.Lock()
	defer rdr.mu.Unlock()

	for chunkId(off-rdr.coff) >= rdr.nreader && !rdr.isClosed {
		db.DPrintf(db.PREADER, "chunkReader: wait for read c %d p %d", chunkId(rdr.coff), chunkId(off))
		rdr.pcond.Wait()
	}
	if rdr.isClosed {
		return 0, false
	}
	return chunkId(off), true
}

func (rdr *AsyncFileReader) doneChunk(i int, sz int, r io.ReadCloser) {
	rdr.mu.Lock()
	defer rdr.mu.Unlock()

	db.DPrintf(db.PREADER, "chunkReader: getChunk %d sz %d err %v", i, sz, rdr.err)

	if sz < sp.BUFSZ { // eof?
		rdr.end = sp.Toffset(chunk2Offset(i) + sz)
	}

	rdr.done[chunk2Buf(i, rdr.nreader)] = true
	rdr.rdrs[chunk2Buf(i, rdr.nreader)] = r
	eq := rdr.poff == rdr.coff
	for j := i; rdr.done[chunk2Buf(j, rdr.nreader)]; j++ {
		if rdr.poff == sp.Toffset(chunk2Offset(j)) {
			rdr.poff += sp.BUFSZ
			rdr.done[chunk2Buf(j, rdr.nreader)] = false
		}
	}

	if eq && rdr.poff > rdr.coff {
		db.DPrintf(db.PREADER, "chunkReader: wakeup reader c %d (p %d)", chunkId(rdr.coff), chunkId(rdr.poff))
		rdr.ccond.Signal()
	}
}

func (rdr *AsyncFileReader) chunkReader() {
	defer rdr.wg.Done()

	for {
		off, ok := rdr.getChunk()
		if !ok {
			break
		}
		db.DPrintf(db.PREADER, "chunkReader: getChunk %d(%d)", chunkId(off), off)
		i, ok := rdr.getBuf(off)
		if !ok {
			break
		}
		l := sp.BUFSZ
		if int(rdr.end-off) < sp.BUFSZ {
			l = int(rdr.end - off)
		}
		r, err := rdr.sof.PreadRdr(rdr.fd, off, sp.Tsize(l))
		if err != nil && err != io.EOF {
			db.DPrintf(db.PREADER, "chunkReader: Pread l %d err %v", l, err)
			rdr.setErr(err)
			break
		}
		db.DPrintf(db.PREADER, "chunkReader: ck %d(%d) read %d err %v", i, off, l, err)
		rdr.doneChunk(i, l, r)
	}
	db.DPrintf(db.PREADER, "chunkReader: done %d", rdr.end)
}

func (fl *FsLib) OpenAsyncReaderRegion(path string, offset sp.Toffset, l sp.Tlength, buf []byte, concurrency int) (*AsyncFileReader, error) {
	fd, err := fl.Open(path, sp.OREAD)
	if err != nil {
		return nil, err
	}
	n := len(buf) / sp.BUFSZ
	if n > 0 && n != concurrency {
		db.DFatalf("OpenAsyncReaderRegion: wrong concurrency")
	}
	r := &AsyncFileReader{
		fd:      fd,
		sof:     fl.FileAPI,
		end:     offset + sp.Toffset(l),
		buf:     buf,
		rdrs:    make([]io.ReadCloser, concurrency),
		nreader: concurrency,
		coff:    offset,
		poff:    offset,
		noff:    offset,
		done:    make([]bool, concurrency),
	}
	r.pcond = sync.NewCond(&r.mu)
	r.ccond = sync.NewCond(&r.mu)
	for i := 0; i < concurrency; i++ {
		r.wg.Add(1)
		go r.chunkReader()
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
