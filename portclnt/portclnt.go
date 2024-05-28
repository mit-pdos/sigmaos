package portclnt

import (
	"path"

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
	return NewPortClntPortPort(fsl, sp.NO_PORT)
}

func NewPortClntPortPort(fsl *fslib.FsLib, p sp.Tport) (*PortClnt, PortInfo, error) {
	pc, err := NewPortClnt(fsl, fsl.ProcEnv().KernelID)
	if err != nil {
		return nil, PortInfo{}, err
	}
	pi, err := pc.AllocPort(p)
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

func (pc *PortClnt) AdvertisePort(pn string, pi PortInfo, net string, lep *sp.Tendpoint) error {
	ep := port.NewPublicEndpoint(pi.HostIP, pi.PBinding, net, lep)
	db.DPrintf(db.PORT, "AdvertisePort %v %v\n", pn, ep)
	if err := pc.MkEndpointFile(pn, ep, sp.NoLeaseId); err != nil {
		return err
	}
	return nil
}

func AdvertisePublicHTTPPort(fsl *fslib.FsLib, pn string, uprocsrvPort sp.Tport, ep *sp.Tendpoint) error {
	// When running with overlays, uprocd's mount point is set up using
	// 127.0.0.1 and the host port (since normally, only the local schedd talks
	// to it). We need to fix up the mount point for the local proc to talk to
	// the uprocd.
	epPN := path.Join(sp.SCHEDD, fsl.ProcEnv().GetKernelID(), sp.UPROCDREL, fsl.ProcEnv().GetUprocdPID().String())
	b, err := fsl.GetFile(epPN)
	if err != nil {
		db.DFatalf("Error get uprocsrv EP: %v", err)
	}
	uprocsrvEP, err := sp.NewEndpointFromBytes(b)
	if err != nil {
		db.DFatalf("Error unmarshal ep for uprocsrv: %v", err)
	}
	uprocsrvEP.Addrs()[0].IPStr = fsl.ProcEnv().GetInnerContainerIP().String()
	uprocsrvEP.Addrs()[0].PortInt = uint32(uprocsrvPort)
	// Manually mount uprocd using the fixed-up endpoint
	if err := fsl.MountTree(uprocsrvEP, "", epPN); err != nil {
		db.DFatalf("Err MountTree: ep %v err %v", uprocsrvEP, err)
	}
	portPN := path.Join(epPN, sp.PUBLIC_PORT)
	// Read the port binding for this uprocd's open port
	var pm port.PortBinding
	if err := fsl.GetFileJson(portPN, &pm); err != nil {
		db.DFatalf("Error get port binding: %v", err)
	}
	ep.Addrs()[0].IPStr = fsl.ProcEnv().GetOuterContainerIP().String()
	ep.Addrs()[0].PortInt = uint32(pm.HostPort)
	db.DPrintf(db.PORT, "AdvertisePortNew %v %v\n", pn, ep)
	return fsl.MkEndpointFile(pn, ep, sp.NoLeaseId)
}
