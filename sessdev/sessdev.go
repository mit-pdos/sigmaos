package sessdev

import (
	"sigmaos/clonedev"
	db "sigmaos/debug"
	"sigmaos/fcall"
	"sigmaos/fs"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

const (
	DATA = "data-"
)

func DataName(fn string) string {
	return DATA + fn
}

type MkSessionF func(*memfssrv.MemFs, fcall.Tsession) (fs.Inode, *fcall.Err)

type SessDev struct {
	mfs *memfssrv.MemFs
	fn  string
	mks MkSessionF
}

func MkSessDev(mfs *memfssrv.MemFs, fn string, mks MkSessionF) error {
	fd := &SessDev{mfs, fn, mks}
	if err := clonedev.MkCloneDev(mfs, fn, fd.mkSession, fd.detachSession); err != nil {
		return err
	}
	return nil
}

// XXX clean up in case of error
func (fd *SessDev) mkSession(mfs *memfssrv.MemFs, sid fcall.Tsession) *fcall.Err {
	sess, err := fd.mks(mfs, sid)
	if err != nil {
		return err
	}
	sidn := clonedev.SidName(sid.String(), fd.fn)
	fn := sidn + "/" + DataName(fd.fn)
	db.DPrintf(db.SESSDEV, "mkSession %v\n", fn)
	if err := mfs.MkDev(fn, sess); err != nil {
		db.DPrintf(db.SESSDEV, "mkSession %v err %v\n", fn, err)
		return err
	}
	return nil
}

func (fd *SessDev) detachSession(sid fcall.Tsession) {
	sidn := clonedev.SidName(sid.String(), fd.fn)
	fn := sidn + "/" + DataName(fd.fn)
	if err := fd.mfs.Remove(fn); err != nil {
		db.DPrintf(db.SESSDEV, "detachSession %v err %v\n", fn, err)
	}
}

func (fd *SessDev) Close(ctx fs.CtxI, m sp.Tmode) *fcall.Err {
	fn := clonedev.SidName(ctx.SessionId().String(), fd.fn) + "/" + DataName(fd.fn)
	db.DPrintf(db.SESSDEV, "%v: Close %v\n", proc.GetName(), fn)
	return nil
}
