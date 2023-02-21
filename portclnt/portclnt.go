package portclnt

import (
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kernelclnt"
	"sigmaos/port"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type PortInfo struct {
	Hip string
	Pb  port.PortBinding
}

type PortClnt struct {
	*fslib.FsLib
	kc *kernelclnt.KernelClnt
}

func MkPortClnt(fsl *fslib.FsLib, kernelId string) (*PortClnt, error) {
	kc, err := kernelclnt.MakeKernelClnt(fsl, sp.BOOT+proc.GetKernelId())
	if err != nil {
		return nil, err
	}
	return &PortClnt{fsl, kc}, nil
}

func MkPortClntPort(sc *sigmaclnt.SigmaClnt) (*PortClnt, PortInfo, error) {
	pc, err := MkPortClnt(sc.FsLib, proc.GetKernelId())
	if err != nil {
		return nil, PortInfo{}, err
	}
	pi, err := pc.AllocPort(port.NOPORT)
	if err != nil {
		return nil, PortInfo{}, err
	}
	return pc, pi, nil
}

func (pc *PortClnt) AllocPort(p port.Tport) (PortInfo, error) {
	hip, pb, err := pc.kc.Port(proc.GetUprocdPid(), p)
	if err != nil {
		return PortInfo{}, err
	}
	db.DPrintf(db.PORT, "hip %v pm %v\n", hip, pb)
	return PortInfo{hip, pb}, nil
}

func (pc *PortClnt) AdvertisePort(pn string, pi PortInfo, net string, laddr string) error {
	mnt := port.MkPublicMount(pi.Hip, pi.Pb, net, laddr)
	db.DPrintf(db.PORT, "AdvertisePort %v %v\n", pn, mnt)
	if err := pc.MkMountSymlink(pn, mnt); err != nil {
		return err
	}
	return nil
}
