package fsux

import (
	"syscall"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type File struct {
	*Obj
	fd int
}

func newFile(path path.Tpathname) (*File, *serr.Err) {
	f := &File{}
	o, err := newObj(path)
	if err != nil {
		return nil, err
	}
	f.Obj = o
	return f, nil
}

func (f *File) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.UX, "%v: FileOpen %v m 0x%x path %v flags 0x%x\n", ctx, f, m, f.Path(), uxFlags(m))
	fd, err := syscall.Open(f.PathName(), uxFlags(m), 0)
	if err != nil {
		return nil, serr.NewErr(serr.TErrError, err)
	}
	f.fd = fd
	return nil, nil
}

func (f *File) Close(ctx fs.CtxI, mode sp.Tmode) *serr.Err {
	db.DPrintf(db.UX, "%v: FileClose %v\n", ctx, f)
	err := syscall.Close(f.fd)
	if err != nil {
		return serr.NewErr(serr.TErrError, err)
	}
	return nil
}

func (f *File) Read(ctx fs.CtxI, off sp.Toffset, cnt sp.Tsize, fence sp.Tfence) ([]byte, *serr.Err) {
	db.DPrintf(db.UX, "%v: Pread: %v off %v cnt %v\n", ctx, f, off, cnt)
	b := make([]byte, cnt)
	n, err := syscall.Pread(f.fd, b, int64(off))
	if err != nil {
		db.DPrintf(db.UX, "Pread %v err %v\n", f, err)
		return nil, serr.NewErr(serr.TErrError, err)
	}
	return b[:n], nil
}

func (f *File) Write(ctx fs.CtxI, off sp.Toffset, b []byte, fence sp.Tfence) (sp.Tsize, *serr.Err) {
	db.DPrintf(db.UX, "%v: Pwrite: off %v cnt %v fence %v\n", f, off, len(b), fence)
	if off == sp.NoOffset {
		// ignore; file was opened with OAPPEND and NoOffset
		// doesn't fit in int64.
		off = 0
	}
	n, err := syscall.Pwrite(f.fd, b, int64(off))
	if err != nil {
		db.DPrintf(db.UX, "Pwrite %v err %v\n", f, err)
		return 0, serr.NewErr(serr.TErrError, err)
	}
	return sp.Tsize(n), nil
}
