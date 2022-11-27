package sessdev

import (
	"sigmaos/clonedev"
	"sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/memfssrv"
	np "sigmaos/ninep"
)

const (
	CLONE = "clone-"
	DATA  = "data-"
)

func Clone(fn string) string {
	return CLONE + fn
}

func Data(fn string) string {
	return DATA + fn
}

type MkSessionF func(*memfssrv.MemFs, np.Tsession) (fs.Inode, *np.Err)

type SessDev struct {
	mfs *memfssrv.MemFs
	fn  string
	mks MkSessionF
}

func MkSessDev(mfs *memfssrv.MemFs, fn string, mks MkSessionF) error {
	fd := &SessDev{mfs, fn, mks}
	if err := clonedev.MkCloneDev(mfs, CLONE+fn, fd.mkSession, fd.detachSession); err != nil {
		return err
	}
	return nil
}

// XXX clean up in case of error
func (fd *SessDev) mkSession(mfs *memfssrv.MemFs, sid np.Tsession) *np.Err {
	sess, err := fd.mks(mfs, sid)
	if err != nil {
		return err
	}
	if err := mfs.MkDev(sid.String()+"/"+DATA+fd.fn, sess); err != nil {
		return err
	}
	return nil
}

func (fd *SessDev) detachSession(sid np.Tsession) {
	if err := fd.mfs.Remove(sid.String() + "/" + DATA + fd.fn); err != nil {
		debug.DPrintf("DBSRV", "detachSessoin err %v\n", err)
	}
}
