package fslib

import (
	"bufio"
	"fmt"
	"io"

	"github.com/klauspost/readahead"

	"sigmaos/awriter"
	db "sigmaos/debug"
	"sigmaos/reader"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/writer"
)

func (fl *FsLib) ReadSeqNo() sessp.Tseqno {
	return fl.FidClnt.ReadSeqNo()
}

//
// Single shot operations
//

func (fl *FsLib) GetFile(fname string) ([]byte, error) {
	return fl.FdClient.GetFile(fname)
}

func (fl *FsLib) SetFile(fname string, data []byte, m sp.Tmode, off sp.Toffset) (sp.Tsize, error) {
	return fl.FdClient.PutFile(fname, 0777, m, data, off, sp.NoLeaseId)
}

func (fl *FsLib) PutFile(fname string, perm sp.Tperm, mode sp.Tmode, data []byte) (sp.Tsize, error) {
	return fl.FdClient.PutFile(fname, perm, mode, data, 0, sp.NoLeaseId)
}

func (fl *FsLib) PutFileEphemeral(fname string, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, data []byte) (sp.Tsize, error) {
	return fl.FdClient.PutFile(fname, perm|sp.DMTMP, mode, data, 0, lid)
}

//
// Open readers
//

func (fl *FsLib) OpenReader(path string) (*reader.Reader, error) {
	fd, err := fl.Open(path, sp.OREAD)
	if err != nil {
		return nil, err
	}
	return fl.NewReader(fd, path), nil
}

type Rdr struct {
	rdr  *reader.Reader
	brdr *bufio.Reader
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
	r := &Rdr{}
	r.rdr = rdr
	if err := rdr.Lseek(offset); err != nil {
		return nil, err
	}
	r.brdr = bufio.NewReaderSize(rdr, sp.BUFSZ)
	r.ardr, err = readahead.NewReaderSize(r.brdr, 4, sp.BUFSZ)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (fl *FsLib) OpenReaderWatch(path string) (*reader.Reader, error) {
	ch := make(chan error)
	fd := -1
	for {
		fd1, err := fl.OpenWatch(path, sp.OREAD, func(path string, err error) {
			ch <- err
		})
		db.DPrintf(db.FSLIB, "OpenWatch %v err %v\n", path, err)
		if serr.IsErrCode(err, serr.TErrNotfound) {
			r := <-ch
			if r != nil {
				db.DPrintf(db.FSLIB, "OpenWatch watch %v err %v\n", path, err)
			}
		} else if err != nil {
			return nil, err
		} else { // success; file is opened
			fd = fd1
			break
		}
	}
	rdr := fl.NewReader(fd, path)
	return rdr, nil

}

func (fl *FsLib) GetFileWatch(path string) ([]byte, error) {
	rdr, err := fl.OpenReaderWatch(path)
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
	wrt  *writer.Writer
	awrt *awriter.Writer
	bwrt *bufio.Writer
}

func (fl *FsLib) CreateAsyncWriter(fname string, perm sp.Tperm, mode sp.Tmode) (*Wrt, error) {
	w, err := fl.CreateWriter(fname, perm, mode)
	if err != nil {
		return nil, err
	}
	aw := awriter.NewWriterSize(w, 4, sp.BUFSZ)
	bw := bufio.NewWriterSize(aw, sp.BUFSZ)
	return &Wrt{w, aw, bw}, nil
}

func (wrt *Wrt) Close() error {
	if err := wrt.bwrt.Flush(); err != nil {
		return err
	}
	if err := wrt.awrt.Close(); err != nil {
		return err
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
	st, err := fl.Stat(src)
	if err != nil {
		return err
	}
	rdr, err := fl.OpenAsyncReader(src, 0)
	if err != nil {
		return err
	}
	defer rdr.Close()
	wrt, err := fl.CreateAsyncWriter(dst, st.Tmode(), 0777)
	if err != nil {
		return err
	}
	defer wrt.Close()
	b := make([]byte, sp.BUFSZ)
	for {
		n, err := rdr.Read(b)
		if err != nil {
			return err
		}
		// Nothing left to read
		if n == 0 {
			break
		}
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
