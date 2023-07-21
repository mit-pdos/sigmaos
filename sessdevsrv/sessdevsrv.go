package sessdevsrv

import (
	"sigmaos/clonedev"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sessdev"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type MkSessionF func(*memfssrv.MemFs, sessp.Tsession) (fs.Inode, *serr.Err)

type SessDev struct {
	mfs *memfssrv.MemFs
	pn  string
	mks MkSessionF
}

// Make a SessDev in mfs at pn
func MkSessDev(mfs *memfssrv.MemFs, pn string, mks MkSessionF, wctl clonedev.WriteCtlF) error {
	db.DPrintf(db.SESSDEV, "MkSessDev: %v\n", pn)
	fd := &SessDev{mfs, pn, mks}
	if err := clonedev.MkCloneDev(mfs, pn, fd.mkSession, fd.detachSession, wctl); err != nil {
		return err
	}
	return nil
}

// XXX clean up in case of error
func (fd *SessDev) mkSession(mfs *memfssrv.MemFs, sid sessp.Tsession) *serr.Err {
	sess, err := fd.mks(mfs, sid)
	if err != nil {
		return err
	}
	sidn := sessdev.SidName(sid.String(), fd.pn)
	fn := sidn + "/" + sessdev.DataName(fd.pn)
	db.DPrintf(db.SESSDEV, "mkSession %v\n", fn)
	if err := mfs.MkDev(fn, sess); err != nil {
		db.DPrintf(db.SESSDEV, "mkSession %v err %v\n", fn, err)
		return err
	}
	return nil
}

func (fd *SessDev) detachSession(sid sessp.Tsession) {
	sidn := sessdev.SidName(sid.String(), fd.pn)
	fn := sidn + "/" + sessdev.DataName(fd.pn)
	if err := fd.mfs.Remove(fn); err != nil {
		db.DPrintf(db.SESSDEV, "detachSession %v err %v\n", fn, err)
	}
}

func (fd *SessDev) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	fn := sessdev.SidName(ctx.SessionId().String(), fd.pn) + "/" + sessdev.DataName(fd.pn)
	db.DPrintf(db.SESSDEV, "%v: Close %v\n", proc.GetName(), fn)
	return nil
}
