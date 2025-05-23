package srv

import (
	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
	"sigmaos/spproto/srv/fid"
	"sigmaos/spproto/srv/watch"
)

func FidWrite(f *fid.Fid, off sp.Toffset, b []byte, fence sp.Tfence) (sp.Tsize, *serr.Err) {
	o := f.Obj()
	var err *serr.Err
	sz := sp.Tsize(0)

	switch i := o.(type) {
	case fs.File:
		sz, err = i.Write(f.Ctx(), off, b, fence)
	default:
		db.DFatalf("Write: obj type %T isn't Dir or File\n", o)
	}
	return sz, err
}

func FidWriteRead(f *fid.Fid, req sessp.IoVec) (sessp.IoVec, *serr.Err) {
	o := f.Obj()
	var err *serr.Err
	var iov sessp.IoVec
	switch i := o.(type) {
	case fs.RPC:
		iov, err = i.WriteRead(f.Ctx(), req)
	default:
		db.DFatalf("Write: obj type %T isn't RPC\n", o)
	}
	return iov, err
}

func readDir(f *fid.Fid, o fs.FsObj, off sp.Toffset, count sp.Tsize) ([]byte, *serr.Err) {
	d := o.(fs.Dir)
	dirents, err := d.ReadDir(f.Ctx(), f.Cursor(), count)
	if err != nil {
		return nil, err
	}
	b, n, e := fs.MarshalDir(count, dirents)
	if err != nil {
		return nil, serr.NewErrError(e)
	}
	f.IncCursor(n)
	return b, nil
}

func FidRead(fidn sp.Tfid, f *fid.Fid, off sp.Toffset, count sp.Tsize, fence sp.Tfence) ([]byte, *serr.Err) {
	switch i := f.Obj().(type) {
	case fs.Dir:
		return readDir(f, f.Obj(), off, count)
	case fs.File:
		b, err := i.Read(f.Ctx(), off, count, fence)
		if err != nil {
			return nil, err
		}
		return b, nil
	case *watch.Watch:
		return i.GetEventBuffer(f, int(count))
	default:
		db.DFatalf("Read: obj %v type %T isn't Dir or File or Watch\n", f.Obj(), f.Obj())
		return nil, nil
	}
}
