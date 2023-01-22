package sigmaclnt

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procclnt"
	sp "sigmaos/sigmap"
)

type SigmaClnt struct {
	*fslib.FsLib
	*procclnt.ProcClnt
}

func MkSigmaClntProc(name string, ip string, namedAddr []string) (*SigmaClnt, error) {
	fsl, err := fslib.MakeFsLibAddr(name, ip, namedAddr)
	if err != nil {
		return nil, err
	}
	pclnt := procclnt.MakeProcClntInit(proc.GenPid(), fsl, name, namedAddr)
	return &SigmaClnt{fsl, pclnt}, nil
}

func MkSigmaClnt(name string) (*SigmaClnt, error) {
	fsl, err := fslib.MakeFsLib(name)
	if err != nil {
		db.DFatalf("MkSigmaClnt: %v", err)
	}
	pclnt := procclnt.MakeProcClnt(fsl)
	return &SigmaClnt{fsl, pclnt}, nil
}

func MkSigmaClntRealmProc(fsl *fslib.FsLib, name string, realm sp.Trealm) (*SigmaClnt, error) {
	pn := path.Join(sp.REALMS, realm.String())
	target, err := fsl.GetFile(pn)
	if err != nil {
		return nil, err
	}
	mnt, r := sp.MkMount(target)
	if r != nil {
		return nil, err
	}
	db.DPrintf(db.REALMCLNT, "mnt %v\n", mnt.Addr)

	return MkSigmaClntProc(name, fsl.GetLocalIP(), mnt.Addr)
}
