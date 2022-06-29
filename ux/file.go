package fsux

import (
	"io"
	"log"
	"os"
	"sync"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/pathmap"
)

// shared among threads using same path
type entry struct {
	sync.Mutex
	v np.TQversion
}

func mkEntry() *entry {
	return &entry{}
}

func (e *entry) version() np.TQversion {
	e.Lock()
	defer e.Unlock()

	return e.v
}

type File struct {
	*Obj
	file *os.File
	pe   *pathmap.Entry
}

func makeFile(path []string) (*File, *np.Err) {
	f := &File{}
	o, err := makeObj(path)
	if err != nil {
		return nil, err
	}
	f.Obj = o
	return f, nil
}

func (f *File) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {
	db.DPrintf("UXD", "%v: Open %v %v path %v flags %v\n", ctx, f, m, f.Path(), uxFlags(m))
	file, err := os.OpenFile(f.Path(), uxFlags(m), 0)
	if err != nil {
		return nil, np.MkErr(np.TErrError, err)
	}
	f.file = file
	f.pe = paths.Insert(f.path, mkEntry())
	return nil, nil
}

func (f *File) Close(ctx fs.CtxI, mode np.Tmode) *np.Err {
	db.DPrintf("UXD", "%v: Close fd for path %v\n", ctx, f.Path())
	err := f.file.Close()
	paths.Delete(f.path)
	if err != nil {
		return np.MkErr(np.TErrError, err)
	}
	return nil
}

func (f *File) uxWrite(off int64, b []byte) (np.Tsize, *np.Err) {
	db.DPrintf("UXD", "%v: WriteFile: off %v cnt %v %v\n", f, off, len(b), f.file)
	_, err := f.file.Seek(off, 0)
	if err != nil {
		return 0, np.MkErr(np.TErrError, err)
	}
	n, err := f.file.Write(b)
	if err != nil {
		return 0, np.MkErr(np.TErrError, err)
	}
	return np.Tsize(n), nil
}

func (f *File) uxRead(off int64, cnt np.Tsize) ([]byte, *np.Err) {
	sz := f.Obj.size()
	if np.Tlength(cnt) >= sz {
		cnt = np.Tsize(sz)
	}
	b := make([]byte, cnt)
	_, err := f.file.Seek(off, 0)
	if err != nil {
		return nil, np.MkErr(np.TErrError, err)
	}
	n, err := f.file.Read(b)
	if err == io.EOF {
		return b[:n], nil
	}
	if err != nil {
		return nil, np.MkErr(np.TErrError, err)
	}
	return b[:n], nil
}

func (f *File) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	db.DPrintf("UXD", "%v: Read: %v off %v cnt %v\n", ctx, f, off, cnt)
	b, err := f.uxRead(int64(off), cnt)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (f *File) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	pe := f.pe.E.(*entry)
	pe.Lock()
	defer pe.Unlock()

	db.DPrintf("UXD0", "%v: Write %v off %v sz %v\n", ctx, f, off, len(b))

	v1 := pe.v
	if !np.VEq(v, v1) {
		log.Printf("v mismatch %v %v\n", v, v1)
		return 0, np.MkErr(np.TErrVersion, f.Qid)
	}
	if off == np.NoOffset {
		// ignore; file was opened with OAPPEND and NoOffset
		// doesn't fit in int64.
		off = 0
	}
	sz, err := f.uxWrite(int64(off), b)
	pe.v += 1
	return sz, err
}
