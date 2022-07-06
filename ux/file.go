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
	db.DPrintf("UXD", "%v: Close %v\n", ctx, f)
	err := syscall.Close(f.fd)
	if err != nil {
		return np.MkErr(np.TErrError, err)
	}
	return nil
}

func (f *File) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	db.DPrintf("UXD", "%v: Pread: %v off %v cnt %v\n", ctx, f, off, cnt)
	b := make([]byte, cnt)
	n, err := syscall.Pread(f.fd, b, int64(off))
	if err != nil {
		db.DPrintf("UXD", "Pread %v err %v\n", f, err)
		return nil, np.MkErr(np.TErrError, err)
	}
	return b[:n], nil
}

func (f *File) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	db.DPrintf("UXD", "%v: Pwrite: off %v cnt %v\n", f, off, len(b))
	if off == np.NoOffset {
		// ignore; file was opened with OAPPEND and NoOffset
		// doesn't fit in int64.
		off = 0
	}
	n, err := syscall.Pwrite(f.fd, b, int64(off))
	if err != nil {
		db.DPrintf("UXD", "Pwrite %v err %v\n", f, err)
		return 0, np.MkErr(np.TErrError, err)
	}
	return np.Tsize(n), nil
}
