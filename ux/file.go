package fsux

import (
	"syscall"

	db "sigmaos/debug"
	"sigmaos/fcall"
	"sigmaos/fs"
	"sigmaos/path"
	sp "sigmaos/sigmap"
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

func (f *File) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *fcall.Err) {
	db.DPrintf(db.UX, "%v: FileOpen %v m 0x%x path %v flags 0x%x\n", ctx, f, m, f.Path(), uxFlags(m))
	fd, err := syscall.Open(f.PathName(), uxFlags(m), 0)
	if err != nil {
		return nil, fcall.MkErr(fcall.TErrError, err)
	}
	f.fd = fd
	return nil, nil
}

func (f *File) Close(ctx fs.CtxI, mode sp.Tmode) *fcall.Err {
	db.DPrintf(db.UX, "%v: FileClose %v\n", ctx, f)
	err := syscall.Close(f.fd)
	if err != nil {
		return fcall.MkErr(fcall.TErrError, err)
	}
	return nil
}

func (f *File) Read(ctx fs.CtxI, off sp.Toffset, cnt fcall.Tsize, v sp.TQversion) ([]byte, *fcall.Err) {
	db.DPrintf(db.UX, "%v: Pread: %v off %v cnt %v\n", ctx, f, off, cnt)
	b := make([]byte, cnt)
	n, err := syscall.Pread(f.fd, b, int64(off))
	if err != nil {
		db.DPrintf(db.UX, "Pread %v err %v\n", f, err)
		return nil, fcall.MkErr(fcall.TErrError, err)
	}
	return b[:n], nil
}

func (f *File) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (fcall.Tsize, *fcall.Err) {
	db.DPrintf(db.UX, "%v: Pwrite: off %v cnt %v\n", f, off, len(b))
	if off == sp.NoOffset {
		// ignore; file was opened with OAPPEND and NoOffset
		// doesn't fit in int64.
		off = 0
	}
	n, err := syscall.Pwrite(f.fd, b, int64(off))
	if err != nil {
		db.DPrintf(db.UX, "Pwrite %v err %v\n", f, err)
		return 0, fcall.MkErr(fcall.TErrError, err)
	}
	return fcall.Tsize(n), nil
}
