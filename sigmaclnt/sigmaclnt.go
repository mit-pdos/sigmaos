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
	pclnt := procclnt.MakeProcClntInit(proc.GetPid(), fsl, name)
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
	pn := path.Join(sp.REALMS, rid.String())
	target, err := rootrealm.GetFile(pn)
	if err != nil {
		return nil, err
	}
	mnt, r := sp.MkMount(target)
	if r != nil {
		return nil, err
	}
	db.DPrintf(db.REALMCLNT, "Realm %v NamedAddr %v\n", rid, mnt.Addr)
	realm, err := fslib.MakeFsLibAddr(name, rootrealm.GetLocalIP(), mnt.Addr)
	return &SigmaClnt{realm, nil}, nil
}

func MkSigmaClntRealmProc(rootfsl *fslib.FsLib, name string, rid sp.Trealm) (*SigmaClnt, error) {
	db.DPrintf(db.REALMCLNT, "MkSigmaClntRealmProc %v\n", rid)
	sc, err := MkSigmaClntRealm(rootfsl, name, rid)
	if err != nil {
		return nil, err
	}
	sc.ProcClnt = procclnt.MakeProcClntInit(proc.GetPid(), sc.FsLib, name)
	return sc, nil
}
