package realmv1

import (
	"sigmaos/bootkernelclnt"
	"sigmaos/fslib"
	"sigmaos/kernelclnt"
	"sigmaos/proc"
	"sigmaos/procclnt"
	sp "sigmaos/sigmap"
)

type Realm struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	boot      *bootkernelclnt.Kernel
	kernel    *kernelclnt.KernelClnt
	namedAddr []string
	Realmid   string
}

func BootRealm(realmid, yml string) (*Realm, error) {
	k, err := bootkernelclnt.BootKernel(realmid, true, yml)
	if err != nil {
		return nil, err
	}
	nameds, err := fslib.SetNamedIP(k.Ip())
	if err != nil {
		return nil, err
	}
	fsl, pclnt, err := mkClient(k.Ip(), realmid, nameds)
	if err != nil {
		return nil, err
	}
	kclnt, err := kernelclnt.MakeKernelClnt(fsl, sp.BOOT+"~local/")
	if err != nil {
		return nil, err
	}
	return &Realm{fsl, pclnt, k, kclnt, nameds, realmid}, nil
}

func (r *Realm) Shutdown() error {
	return r.boot.Shutdown()
}

func (r *Realm) Boot(s string) error {
	return r.kernel.Boot(s)
}

func (r *Realm) KillOne(s string) error {
	return r.kernel.Kill(s)
}

func (r *Realm) NamedAddr() []string {
	return r.namedAddr
}

func (r *Realm) GetIP() string {
	return r.boot.Ip()
}

func mkClient(kip string, realmid string, namedAddr []string) (*fslib.FsLib, *procclnt.ProcClnt, error) {
	fsl, err := fslib.MakeFsLibAddr("test", kip, namedAddr)
	if err != nil {
		return nil, nil, err
	}
	pclnt := procclnt.MakeProcClntInit(proc.GenPid(), fsl, "test", namedAddr)
	return fsl, pclnt, nil
}
