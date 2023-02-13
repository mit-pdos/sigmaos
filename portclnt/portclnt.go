package portclnt

import (
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kernelclnt"
	"sigmaos/port"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

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

func (pc *PortClnt) AllocPort(p port.Tport) (string, port.PortBinding, error) {
	hip, pb, err := pc.kc.Port(proc.GetUprocdPid(), p)
	if err != nil {
		return "", port.PortBinding{}, err
	}
	db.DPrintf(db.PORT, "hip %v pm %v\n", hip, pb)
	return hip, pb, nil
}

func (pc *PortClnt) AdvertisePort(pn string, hip string, pb port.PortBinding, laddr string) error {

	addrs := sp.MkTaddrs([]string{laddr, hip + ":" + pb.HostPort.String()})
	mnt := sp.MkMountService(addrs)
	db.DPrintf(db.PORT, "AdvertisePort %v %v\n", pn, mnt)
	if err := pc.MkMountSymlink(pn, mnt); err != nil {
		return err
	}
	return nil
}
