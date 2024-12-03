package namesrv

import (
	db "sigmaos/debug"
	"sigmaos/api/fs"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type File struct {
	*Obj
}

func newFile(o *Obj) *File {
	f := &File{Obj: o}
	return f
}

func (f *File) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.NAMED, "%v: FileOpen %v m 0x%x path %v\n", ctx.ClntId(), f, m, f.Obj.pn)
	if f.Obj.di.Nf == nil {
		nf, _, c, err := f.Obj.fs.GetFile(&f.Obj.di)
		f.Obj.fs.PstatUpdate(f.pn, c)
		if err != nil {
			return nil, err
		}
		f.Obj.di.Nf = nf
	}
	return nil, nil
}

func (f *File) Close(ctx fs.CtxI, mode sp.Tmode) *serr.Err {
	db.DPrintf(db.NAMED, "%v: FileClose %v\n", ctx.ClntId(), f)
	return nil
}

// XXX maybe do get
func (f *File) Read(ctx fs.CtxI, offset sp.Toffset, n sp.Tsize, fence sp.Tfence) ([]byte, *serr.Err) {
	db.DPrintf(db.NAMED, "%v: FileRead: %v off %v cnt %v\n", ctx.ClntId(), f, offset, n)
	if offset >= f.LenOff() {
		return nil, nil
	} else {
		// XXX overflow?
		end := offset + sp.Toffset(n)
		if end >= f.LenOff() {
			end = f.LenOff()
		}
		b := f.di.Nf.Data[offset:end]
		return b, nil
	}
}

func (f *File) LenOff() sp.Toffset {
	return sp.Toffset(len(f.Obj.di.Nf.Data))
}

func (f *File) Write(ctx fs.CtxI, offset sp.Toffset, b []byte, fence sp.Tfence) (sp.Tsize, *serr.Err) {
	db.DPrintf(db.NAMED, "%v: Write: %v off %v cnt %v fence %v\n", ctx.ClntId(), f, offset, len(b), fence)
	cnt := sp.Tsize(len(b))
	sz := sp.Toffset(len(b))

	if offset == sp.NoOffset { // OAPPEND
		offset = f.LenOff()
	}

	if offset >= f.LenOff() { // passed end of file?
		n := f.LenOff() - offset
		new := append(f.Obj.di.Nf.Data, make([]byte, n)...)
		new = append(new, b...)

		if err := f.Obj.putObj(fence, new); err != nil {
			db.DPrintf(db.NAMED, "%v: Write: off %v fence %v err %v", f, offset, fence, err)
			return 0, err
		}
		f.Obj.di.Nf.Data = new
		return cnt, nil
	}

	var d []byte
	if offset+sz < f.LenOff() { // in the middle of the file?
		d = f.Obj.di.Nf.Data[offset+sz:]
	}
	new := f.Obj.di.Nf.Data[0:offset]
	new = append(new, b...)
	new = append(new, d...)

	if err := f.Obj.putObj(fence, new); err != nil {
		db.DPrintf(db.NAMED, "%v: Write: off %v fence %v err %v", f, offset, len(b), err)
		return 0, err
	}
	f.Obj.di.Nf.Data = new
	return sp.Tsize(len(b)), nil
}
