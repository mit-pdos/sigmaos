package fsux

import (
	"io"
	"os"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
)

type File struct {
	*Obj
	file *os.File
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
	db.DLPrintf("UXD", "%v: Open %v %v path %v flags %v\n", ctx, f, m, f.Path(), uxFlags(m))
	file, err := os.OpenFile(f.Path(), uxFlags(m), 0)
	if err != nil {
		return nil, np.MkErr(np.TErrError, err)
	}
	f.file = file
	return nil, nil
}

func (f *File) Close(ctx fs.CtxI, mode np.Tmode) *np.Err {
	db.DLPrintf("UXD", "%v: Close fd for path %v\n", ctx, f.Path())
	err := f.file.Close()
	if err != nil {
		return np.MkErr(np.TErrError, err)
	}
	return nil
}

func (f *File) uxWrite(off int64, whence int, b []byte) (np.Tsize, *np.Err) {
	db.DLPrintf("UXD", "%v: WriteFile: off %v whence %v cnt %v %v\n", f, off, whence, len(b), f.file)
	_, err := f.file.Seek(off, whence)
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
	db.DLPrintf("UXD", "%v: Read: %v off %v cnt %v\n", ctx, f, off, cnt)
	b, err := f.uxRead(int64(off), cnt)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (f *File) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	db.DLPrintf("UXD0", "%v: Write %v off %v sz %v\n", ctx, f, off, len(b))
	whence := 0
	if off == np.NoOffset {
		off = 0
		whence = 2
	}
	return f.uxWrite(int64(off), whence, b)
}
