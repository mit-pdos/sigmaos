package namedv1

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type File struct {
	*Obj
}

func makeFile(o *Obj) *File {
	f := &File{}
	f.Obj = o
	return f
}

func (f *File) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.NAMEDV1, "%v: FileOpen %v m 0x%x path %v\n", ctx, f, m, f.Obj.pn)
	return nil, nil
}

func (f *File) Close(ctx fs.CtxI, mode sp.Tmode) *serr.Err {
	db.DPrintf(db.NAMEDV1, "%v: FileClose %v\n", ctx, f)
	return nil
}

// XXX maybe do get
func (f *File) Read(ctx fs.CtxI, off sp.Toffset, cnt sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	db.DPrintf(db.NAMEDV1, "%v: Read: %v off %v cnt %v\n", ctx, f, off, cnt)
	n := int(cnt)
	if n > len(f.Obj.data) {
		n = len(f.Obj.data)
	}
	b := make([]byte, n-int(off))
	copy(b, f.Obj.data[off:])
	return b, nil
}

func (f *File) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *serr.Err) {
	db.DPrintf(db.NAMEDV1, "%v: Write: off %v cnt %v\n", f, off, len(b))
	if off == sp.NoOffset {
		db.DFatalf("Unimplemented")
	}
	f.Obj.data = b // XXX update right part
	if err := f.Obj.putObj(); err != nil {
		return 0, err
	}
	return sessp.Tsize(len(b)), nil
}
