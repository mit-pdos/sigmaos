package fslib

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/proc"
	"sigmaos/s3/s3pathclnt"
	"sigmaos/serr"
	sos "sigmaos/sigmaos"
	sp "sigmaos/sigmap"
)

type FsLib struct {
	pe  *proc.ProcEnv
	npc *dialproxyclnt.DialProxyClnt
	sos.FileAPI

	sync.Mutex
	s3c  *s3pathclnt.S3PathClnt
	Stop bool
}

func NewFsLibAPI(pe *proc.ProcEnv, npc *dialproxyclnt.DialProxyClnt, sos sos.FileAPI) (*FsLib, error) {
	db.DPrintf(db.FSLIB, "NewFsLib: principal %s innerip %s addrs %v\n", pe.GetPrincipal(), pe.GetInnerContainerIP(), pe.GetEtcdEndpoints())
	fl := &FsLib{
		pe:      pe,
		npc:     npc,
		FileAPI: sos,
	}
	return fl, nil
}

func (fs *FsLib) String() string {
	return fmt.Sprintf("{Fslib npc %v FileAPI %v}", fs.npc, fs.FileAPI)
}

func (fl *FsLib) GetInnerContainerIP() sp.Tip {
	return fl.pe.GetInnerContainerIP()
}

func (fl *FsLib) ProcEnv() *proc.ProcEnv {
	return fl.pe
}

// TODO: should probably remove, and replace by a high-level SigmaOS API call.
func (fl *FsLib) GetDialProxyClnt() *dialproxyclnt.DialProxyClnt {
	return fl.npc
}

func (fl *FsLib) MountTree(ep *sp.Tendpoint, tree, mount string) error {
	return fl.FileAPI.MountTree(ep, tree, mount)
}

func (fl *FsLib) MountS3PathClnt() error {
	var ok bool
	s3secrets, ok := fl.pe.GetSecrets()["s3"]
	if ok {
		fl.Lock()
		defer fl.Unlock()
		if fl.s3c == nil {
			s3c, err := s3pathclnt.NewS3PathClnt(s3secrets, fl.npc)
			if err != nil {
				return err
			}
			if err := fl.MountPathClnt(sp.S3CLNT, s3c); err != nil {
				return err
			}
			fl.s3c = s3c
		}
		return nil
	}
	return serr.NewErr(serr.TErrPerm, fmt.Errorf("Principal has no S3 secrets"))
}

func (fl *FsLib) Close() error {
	return fl.FileAPI.Close()
}

func (fl *FsLib) GetSigmaOS() sos.FileAPI {
	return fl.FileAPI
}
