package fsux

import (
	"syscall"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
)

type File struct {
	*Obj
	fd int
}

func makeFile(path np.Path) (*File, *np.Err) {
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
	fd, err := syscall.Open(f.PathName(), uxFlags(m)|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, np.MkErr(np.TErrError, err)
	}
	f.fd = fd
	return nil, nil
}

func (f *File) Close(ctx fs.CtxI, mode np.Tmode) *np.Err {
	db.DPrintf("UXD", "%v: Close fd for path %v\n", ctx, f.Path())
	err := syscall.Close(f.fd)
	if err != nil {
		return np.MkErr(np.TErrError, err)
	}
	return nil
}

// XXX use pwrite
func (f *File) uxWrite(off int64, b []byte) (np.Tsize, *np.Err) {
	db.DPrintf("UXD", "%v: WriteFile: off %v cnt %v %v\n", f, off, len(b), f.fd)
	n, err := syscall.Pwrite(f.fd, b, off)
	if err != nil {
		return 0, np.MkErr(np.TErrError, err)
	}
	return np.Tsize(n), nil
}

// XXX use pread
func (f *File) uxRead(off int64, cnt np.Tsize) ([]byte, *np.Err) {
	b := make([]byte, cnt)
	n, err := syscall.Pread(f.fd, b, off)
	if err != nil {
		return nil, np.MkErr(np.TErrError, err)
	}
	if n == 0 {
		return nil, nil
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
	db.DPrintf("UXD0", "%v: Write %v off %v sz %v\n", ctx, f, off, len(b))
	if off == np.NoOffset {
		// ignore; file was opened with OAPPEND and NoOffset
		// doesn't fit in int64.
		off = 0
	}
	sz, err := f.uxWrite(int64(off), b)
	return sz, err
}
