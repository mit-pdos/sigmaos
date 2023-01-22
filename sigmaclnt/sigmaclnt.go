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

func MkSigmaClntRealm(rootrealm *fslib.FsLib, name string, rid sp.Trealm) (*SigmaClnt, error) {
	db.DPrintf(db.REALMCLNT, "MkSigmaClntRealm %v\n", rid)
	pn := path.Join(sp.REALMS, rid.String())
	target, err := rootrealm.GetFile(pn)
	if err != nil {
		return nil, err
	}
	mnt, r := sp.MkMount(target)
	if r != nil {
		return nil, err
	}
	db.DPrintf(db.REALMCLNT, "mnt %v\n", mnt.Addr)
	realm, err := fslib.MakeFsLibAddr(name, rootrealm.GetLocalIP(), mnt.Addr)
	return &SigmaClnt{realm, nil}, nil
}

func MkSigmaClntRealmProc(rootrealm *fslib.FsLib, name string, rid sp.Trealm) (*SigmaClnt, error) {
	db.DPrintf(db.REALMCLNT, "MkSigmaClntRealmProc %v\n", rid)
	sc, err := MkSigmaClntRealm(rootrealm, name, rid)
	if err != nil {
		return nil, err
	}
	// pclnt := procclnt.MakeProcClntInit(proc.GenPid(), rootfsl, name, mnt.Addr)
	return sc, nil
}
