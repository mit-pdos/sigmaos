package fslib

import (
	"bufio"
	"fmt"
	"io"

	"github.com/klauspost/readahead"

	"sigmaos/awriter"
	db "sigmaos/debug"
	np "sigmaos/sigmap"
    "sigmaos/fcall"
	"sigmaos/reader"
	"sigmaos/writer"
)

func (fl *FsLib) ReadSeqNo() np.Tseqno {
	return fl.FidClnt.ReadSeqNo()
}

//
// Single shot operations
//

func (fl *FsLib) GetFile(fname string) ([]byte, error) {
	return fl.FdClient.GetFile(fname, np.OREAD, 0, np.MAXGETSET)
}

func (fl *FsLib) SetFile(fname string, data []byte, m np.Tmode, off np.Toffset) (np.Tsize, error) {
	return fl.FdClient.SetFile(fname, m, data, off)
}

func (fl *FsLib) PutFile(fname string, perm np.Tperm, mode np.Tmode, data []byte) (np.Tsize, error) {
	return fl.FdClient.PutFile(fname, mode|np.OWRITE, perm, data, 0)
}

//
// Open readers
//

func (fl *FsLib) OpenReader(path string) (*reader.Reader, error) {
	fd, err := fl.Open(path, np.OREAD)
	if err != nil {
		return nil, err
	}
	return fl.MakeReader(fd, path, fl.GetChunkSz()), nil
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

func (rdr *Rdr) Nbytes() np.Tlength {
	return rdr.rdr.Nbytes()
}

func (fl *FsLib) OpenAsyncReader(path string, offset np.Toffset) (*Rdr, error) {
	rdr, err := fl.OpenReader(path)
	if err != nil {
		return nil, err
	}
	r := &Rdr{}
	r.rdr = rdr
	if err := rdr.Lseek(offset); err != nil {
		return nil, err
	}
	r.brdr = bufio.NewReaderSize(rdr, np.BUFSZ)
	r.ardr, err = readahead.NewReaderSize(r.brdr, 4, np.BUFSZ)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (fl *FsLib) OpenReaderWatch(path string) (*reader.Reader, error) {
	ch := make(chan error)
	fd := -1
	for {
		fd1, err := fl.OpenWatch(path, np.OREAD, func(path string, err error) {
			ch <- err
		})
		db.DPrintf("FSLIB", "OpenWatch %v err %v\n", path, err)
		if err != nil && fcall.IsErrNotfound(err) {
			r := <-ch
			if r != nil {
				db.DPrintf("FSLIB", "OpenWatch watch %v err %v\n", path, err)
			}
		} else if err != nil {
			return nil, err
		} else { // success; file is opened
			fd = fd1
			break
		}
	}
	rdr := fl.MakeReader(fd, path, fl.GetChunkSz())
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

func (fl *FsLib) CreateWriter(fname string, perm np.Tperm, mode np.Tmode) (*writer.Writer, error) {
	fd, err := fl.Create(fname, perm, mode)
	if err != nil {
		return nil, err
	}
	wrt := fl.MakeWriter(fd)
	return wrt, nil
}

func (fl *FsLib) OpenWriter(fname string, mode np.Tmode) (*writer.Writer, error) {
	fd, err := fl.Open(fname, mode)
	if err != nil {
		return nil, err
	}
	wrt := fl.MakeWriter(fd)
	return wrt, nil
}

type Wrt struct {
	wrt  *writer.Writer
	awrt *awriter.Writer
	bwrt *bufio.Writer
}

func (fl *FsLib) CreateAsyncWriter(fname string, perm np.Tperm, mode np.Tmode) (*Wrt, error) {
	w, err := fl.CreateWriter(fname, perm, mode)
	if err != nil {
		return nil, err
	}
	aw := awriter.NewWriterSize(w, np.BUFSZ)
	bw := bufio.NewWriterSize(aw, np.BUFSZ)
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

func (wrt *Wrt) Nbytes() np.Tlength {
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
	fdsrc, err := fl.Open(src, np.OREAD)
	if err != nil {
		return err
	}
	defer fl.Close(fdsrc)
	fddst, err := fl.Create(dst, st.Mode, np.OWRITE)
	if err != nil {
		return err
	}
	defer fl.Close(fddst)
	for {
		b, err := fl.Read(fdsrc, fl.GetChunkSz())
		if err != nil {
			return err
		}
		if len(b) == 0 {
			break
		}
		n, err := fl.Write(fddst, b)
		if err != nil {
			return err
		}
		if n != np.Tsize(len(b)) {
			return fmt.Errorf("short write")
		}
	}
	return nil
}
