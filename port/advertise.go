package port

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/fslib"
	sp "sigmaos/sigmap"
)

type PortInfo struct {
	HostIP   sp.Tip
	PBinding PortBinding
}

func AdvertisePublicHTTPPort(fsl *fslib.FsLib, pn string, ep *sp.Tendpoint) error {
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
	uprocsrvEP.Addrs()[0].PortInt = uint32(UPROCD_PORT)
	// Manually mount uprocd using the fixed-up endpoint
	if err := fsl.MountTree(uprocsrvEP, "", epPN); err != nil {
		db.DFatalf("Err MountTree: ep %v err %v", uprocsrvEP, err)
	}
	portPN := path.Join(epPN, sp.PUBLIC_PORT)
	// Read the port binding for this uprocd's open port
	var pm PortBinding
	if err := fsl.GetFileJson(portPN, &pm); err != nil {
		db.DFatalf("Error get port binding: %v", err)
	}
	ep.Addrs()[0].IPStr = fsl.ProcEnv().GetOuterContainerIP().String()
	ep.Addrs()[0].PortInt = uint32(pm.HostPort)
	db.DPrintf(db.PORT, "AdvertisePortNew %v %v\n", pn, ep)
	return fsl.MkEndpointFile(pn, ep)
}
