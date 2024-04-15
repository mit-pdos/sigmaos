package portclnt

import (
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kernelclnt"
	"sigmaos/port"
	sp "sigmaos/sigmap"
)

type PortInfo struct {
	HostIP   sp.Tip
	PBinding port.PortBinding
}

type PortClnt struct {
	*fslib.FsLib
	kc *kernelclnt.KernelClnt
}

func NewPortClnt(fsl *fslib.FsLib, kernelId string) (*PortClnt, error) {
	kc, err := kernelclnt.NewKernelClnt(fsl, sp.BOOT+fsl.ProcEnv().KernelID)
	if err != nil {
		return nil, err
	}
	return &PortClnt{fsl, kc}, nil
}

func NewPortClntPort(fsl *fslib.FsLib) (*PortClnt, PortInfo, error) {
	pc, err := NewPortClnt(fsl, fsl.ProcEnv().KernelID)
	if err != nil {
		return nil, PortInfo{}, err
	}
	pi, err := pc.AllocPort(sp.NO_PORT)
	if err != nil {
		return nil, PortInfo{}, err
	}
	return pc, pi, nil
}

func (pc *PortClnt) AllocPort(p sp.Tport) (PortInfo, error) {
	hip, pb, err := pc.kc.Port(pc.ProcEnv().GetUprocdPID(), p)
	if err != nil {
		return PortInfo{}, err
	}
	db.DPrintf(db.PORT, "hip %v pm %v\n", hip, pb)
	return PortInfo{hip, pb}, nil
}

func (pc *PortClnt) AdvertisePort(pn string, pi PortInfo, net string, lmnt *sp.Tmount) error {
	mnt := port.NewPublicMount(pi.HostIP, pi.PBinding, net, lmnt)
	db.DPrintf(db.PORT, "AdvertisePort %v %v\n", pn, mnt)
	if err := pc.MkMountFile(pn, mnt, sp.NoLeaseId); err != nil {
		return err
	}
	return nil
}
