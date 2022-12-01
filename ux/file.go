package fsux

import (
	"syscall"

	db "sigmaos/debug"
	"sigmaos/fs"
	np "sigmaos/sigmap"
    "sigmaos/path"
    "sigmaos/fcall"
)

type File struct {
	*Obj
	fd int
}

func makeFile(path path.Path) (*File, *fcall.Err) {
	f := &File{}
	o, err := makeObj(path)
	if err != nil {
		return nil, err
	}
	f.Obj = o
	return f, nil
}

func (f *File) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *fcall.Err) {
	db.DPrintf("UXD", "%v: FileOpen %v m 0x%x path %v flags 0x%x\n", ctx, f, m, f.Path(), uxFlags(m))
	fd, err := syscall.Open(f.PathName(), uxFlags(m), 0)
	if err != nil {
		return nil, fcall.MkErr(fcall.TErrError, err)
	}
	f.fd = fd
	return nil, nil
}

func (f *File) Close(ctx fs.CtxI, mode np.Tmode) *fcall.Err {
	db.DPrintf("UXD", "%v: FileClose %v\n", ctx, f)
	err := syscall.Close(f.fd)
	if err != nil {
		return fcall.MkErr(fcall.TErrError, err)
	}
	return nil
}

func (f *File) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *fcall.Err) {
	db.DPrintf("UXD", "%v: Pread: %v off %v cnt %v\n", ctx, f, off, cnt)
	b := make([]byte, cnt)
	n, err := syscall.Pread(f.fd, b, int64(off))
	if err != nil {
		db.DPrintf("UXD", "Pread %v err %v\n", f, err)
		return nil, fcall.MkErr(fcall.TErrError, err)
	}
	return b[:n], nil
}

func (f *File) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *fcall.Err) {
	db.DPrintf("UXD", "%v: Pwrite: off %v cnt %v\n", f, off, len(b))
	if off == np.NoOffset {
		// ignore; file was opened with OAPPEND and NoOffset
		// doesn't fit in int64.
		off = 0
	}
	n, err := syscall.Pwrite(f.fd, b, int64(off))
	if err != nil {
		db.DPrintf("UXD", "Pwrite %v err %v\n", f, err)
		return 0, fcall.MkErr(fcall.TErrError, err)
	}
	return np.Tsize(n), nil
}
