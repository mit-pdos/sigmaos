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
	o := f.Pobj().Obj()
	var err *serr.Err
	sz := sp.Tsize(0)

	switch i := o.(type) {
	case fs.File:
		sz, err = i.Write(f.Pobj().Ctx(), off, b, fence)
	default:
		db.DFatalf("Write: obj type %T isn't Dir or File\n", o)
	}
	return sz, err
}

func FidWriteRead(f *fid.Fid, req sessp.IoVec) (sessp.IoVec, *serr.Err) {
	o := f.Pobj().Obj()
	var err *serr.Err
	var iov sessp.IoVec
	switch i := o.(type) {
	case fs.RPC:
		iov, err = i.WriteRead(f.Pobj().Ctx(), req)
	default:
		db.DFatalf("Write: obj type %T isn't RPC\n", o)
	}
	return iov, err
}

func readDir(f *fid.Fid, o fs.FsObj, off sp.Toffset, count sp.Tsize) ([]byte, *serr.Err) {
	d := o.(fs.Dir)
	dirents, err := d.ReadDir(f.Pobj().Ctx(), f.Cursor(), count)
	if err != nil {
		return nil, err
	}
	b, n, err := fs.MarshalDir(count, dirents)
	if err != nil {
		return nil, err
	}
	f.IncCursor(n)
	return b, nil
}

func FidRead(f *fid.Fid, off sp.Toffset, count sp.Tsize, fence sp.Tfence) ([]byte, *serr.Err) {
	po := f.Pobj()
	switch i := po.Obj().(type) {
	case fs.Dir:
		return readDir(f, po.Obj(), off, count)
	case fs.File:
		b, err := i.Read(po.Ctx(), off, count, fence)
		if err != nil {
			return nil, err
		}
		return b, nil
	case *watch.WatchV2:
		return i.GetEventBuffer(f, int(count))
	default:
		db.DFatalf("Read: obj %v type %T isn't Dir or File or Watch\n", po.Obj(), po.Obj())
		return nil, nil
	}
}