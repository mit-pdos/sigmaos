package bootkernelclnt

import (
	"os/exec"

	db "sigmaos/debug"
	"sigmaos/kernelclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

//
// Library to start kernel
//

const (
	START = "../start.sh"
)

func Start(srvs string, namedAddr sp.Taddrs) (string, error) {
	out, err := exec.Command(START, []string{
		"--boot", srvs,
		"--named", namedAddr.String()}...).Output()
	if err != nil {
		db.DPrintf(db.BOOT, "Boot failed %s err %v\n", string(out), err)
		return "", err
	}
	ip := string(out)
	db.DPrintf("BOOT", "Start: %v IP %v\n", srvs, ip)
	return ip, nil
}

type Kernel struct {
	*sigmaclnt.SigmaClnt
	kclnt *kernelclnt.KernelClnt
}

func MkBootKernelClnt(name string, conf string, namedAddr sp.Taddrs) (*Kernel, error) {
	ip, err := Start(conf, namedAddr)
	if err != nil {
		return nil, err
	}
	sc, err := sigmaclnt.MkSigmaClntProc(name, ip, namedAddr)
	if err != nil {
		return nil, err
	}
	kclnt, err := kernelclnt.MakeKernelClnt(sc.FsLib, sp.BOOT+"~local/")
	if err != nil {
		return nil, err
	}
	return &Kernel{sc, kclnt}, nil
}

func MkKernelClnt(name string, ip string, namedAddr sp.Taddrs) (*Kernel, error) {
	sc, err := sigmaclnt.MkSigmaClntProc(name, ip, namedAddr)
	if err != nil {
		return nil, err
	}
	kclnt, err := kernelclnt.MakeKernelClnt(sc.FsLib, sp.BOOT+"~local/")
	if err != nil {
		return nil, err
	}
	return &Kernel{sc, kclnt}, nil
}

func (k *Kernel) Shutdown() error {
	return k.kclnt.Shutdown()
}

func (k *Kernel) Boot(s string) error {
	_, err := k.kclnt.Boot(s, sp.Taddrs{})
	return err
}

func (k *Kernel) Kill(s string) error {
	return k.kclnt.Kill(s)
}
