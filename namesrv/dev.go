package namesrv

import (
	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type Dev struct {
	*Obj
}

func newDev(o *Obj) fs.FsObj {
	f := &Dev{Obj: o}
	return f
}

func (d *Dev) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.NAMED, "%v: DevOpen %v m 0x%x path %v\n", ctx, d, m, d.Obj.pn)
	if d.Obj.di.Nf == nil {
		nf, _, c, err := d.Obj.fs.GetFile(&d.Obj.di)
		d.Obj.fs.PstatUpdate(d.Obj.pn, c)
		if err != nil {
			return nil, err
		}
		d.Obj.di.Nf = nf
	}
	return nil, nil
}

func (f *Dev) Close(ctx fs.CtxI, mode sp.Tmode) *serr.Err {
	db.DPrintf(db.NAMED, "%v: DevClose %v\n", ctx, f)
	return nil
}

func (d *Dev) Read(ctx fs.CtxI, offset sp.Toffset, n sp.Tsize, f sp.Tfence) ([]byte, *serr.Err) {
	db.DPrintf(db.NAMED, "%v: DevRead: %v off %v cnt %v key %v\n", ctx, d, offset, n, d.Obj.di.Nf.Data)
	if offset > 0 {
		return nil, nil
	}
	return d.Obj.di.Nf.Data, nil
}

func (d *Dev) Write(ctx fs.CtxI, offset sp.Toffset, b []byte, f sp.Tfence) (sp.Tsize, *serr.Err) {
	db.DPrintf(db.NAMED, "%v: DevWrite: %v off %v cnt %v\n", ctx, d, offset, len(b))
	if err := d.Obj.putObj(f, b); err != nil {
		return 0, err
	}
	d.Obj.di.Nf.Data = b
	return sp.Tsize(len(b)), nil
}

// Methods for Inode interface

func (o *Obj) Mtime() int64 {
	return 0
}

func (o *Obj) SetMtime(m int64) {
}

func (o *Obj) Unlink() {
}
