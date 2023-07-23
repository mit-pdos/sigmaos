package sessdevsrv

import (
	"path"

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
	dir string
	mks MkSessionF
}

// Make a SessDev in mfs in the directory pn
func MkSessDev(mfs *memfssrv.MemFs, dir string, mks MkSessionF, wctl clonedev.WriteCtlF) error {
	db.DPrintf(db.SESSDEV, "MkSessDev: %v\n", dir)
	sd := &SessDev{mfs, dir, mks}
	if err := clonedev.MkCloneDev(mfs, dir, sd.mkSession, sd.detachSession, wctl); err != nil {
		return err
	}
	return nil
}

// XXX clean up in case of error
func (sd *SessDev) mkSession(mfs *memfssrv.MemFs, sid sessp.Tsession) *serr.Err {
	sess, err := sd.mks(mfs, sid)
	if err != nil {
		return err
	}
	fn := path.Join(sd.dir, sid.String(), sessdev.DATA)
	db.DPrintf(db.SESSDEV, "mkSession %v\n", fn)
	if err := mfs.MkDev(fn, sess); err != nil {
		db.DPrintf(db.SESSDEV, "mkSession %v err %v\n", fn, err)
		return err
	}
	return nil
}

func (sd *SessDev) detachSession(sid sessp.Tsession) {
	fn := path.Join(sd.dir, sid.String(), sessdev.DATA)
	if err := sd.mfs.Remove(fn); err != nil {
		db.DPrintf(db.SESSDEV, "detachSession %v err %v\n", fn, err)
	}
}

func (sd *SessDev) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	fn := path.Join(sd.dir, ctx.SessionId().String(), sessdev.DATA)
	db.DPrintf(db.SESSDEV, "%v: Close %v\n", proc.GetName(), fn)
	return nil
}
