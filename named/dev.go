package named

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type Dev struct {
	*Obj
}

func makeDev(o *Obj) fs.FsObj {
	f := &Dev{Obj: o}
	return f
}

func (d *Dev) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.NAMED, "%v: DevOpen %v m 0x%x path %v\n", ctx, d, m, d.Obj.pn)
	if d.Obj.di.Nf == nil {
		nf, _, err := d.Obj.fs.GetFile(d.Obj.di.Path)
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

func (d *Dev) Read(ctx fs.CtxI, offset sp.Toffset, n sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	db.DPrintf(db.NAMED, "%v: DevRead: %v off %v cnt %v key %v\n", ctx, d, offset, n, d.Obj.di.Nf.Data)
	if offset > 0 {
		return nil, nil
	}
	return d.Obj.di.Nf.Data, nil
}

func (d *Dev) Write(ctx fs.CtxI, offset sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *serr.Err) {
	db.DPrintf(db.NAMED, "%v: DevWrite: %v off %v cnt %v\n", ctx, d, offset, len(b))
	d.Obj.di.Nf.Data = b
	if err := d.Obj.putObj(); err != nil {
		return 0, err
	}

	return sessp.Tsize(len(b)), nil
}

func (nd *Named) CreateElectionInfo(pn string) error {
	db.DPrintf(db.NAMED, "CreateElectionInfo %v\n", pn)
	if err := nd.MkDir(path.Join(sp.NAME, path.Dir(pn)), 0777); err != nil {
		db.DPrintf(db.NAMED, "CreateElectionInfo MkDir %v err %v\n", path.Dir(pn), err)
	}
	fd, err := nd.Create(path.Join(sp.NAME, pn), 0777|sp.DMDEVICE, sp.OWRITE)
	if err != nil {
		return err
	}
	if _, err := nd.Write(fd, []byte(pn)); err != nil {
		return err
	}
	return nd.Close(fd)
}
