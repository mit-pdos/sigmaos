package test

import (
	"os"

	bootclnt "sigmaos/boot/clnt"
	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/proc"
	spproxysrv "sigmaos/proxy/sigmap/srv"
	sp "sigmaos/sigmap"
)

type DialProxyProvider struct {
	spkc *bootclnt.Kernel
	spss kernel.Subsystem
}

// Start a new spproxyd/dialproxy, whether that be as a kernel (in its own
// container) or as a Linux process (if the test is running in a container)
func NewDialProxyProvider(pe *proc.ProcEnv, useDialProxy, bootAsKernel bool) (*DialProxyProvider, error) {
	var spss kernel.Subsystem
	var spkc *bootclnt.Kernel
	if bootAsKernel {
		sckid := sp.SPProxydKernel(bootclnt.GenKernelId())
		_, err := bootclnt.Start(sckid, sp.Tip(EtcdIP), pe, sp.SPPROXYDREL, useDialProxy, homeDir, projectRoot, User, netname)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error start kernel for spproxyd")
			return nil, err
		}
		spkc, err = bootclnt.NewKernelClnt(sckid, sp.Tip(EtcdIP), pe)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error make kernel clnt for spproxyd")
			return nil, err
		}
	} else {
		os.Remove(sp.SIGMA_DIALPROXY_SOCKET)
		p := proc.NewPrivProcPid(sp.GenPid("spproxy"), "spproxy", nil, true)
		p.GetProcEnv().SetSecrets(pe.GetSecrets())
		p.SetHow(proc.HLINUX)
		p.InheritParentProcEnv(pe)
		p.GetProcEnv().UseDialProxy = false
		// Add kernel bin dir to path so exec can find the spproxy bin
		path := os.Getenv("PATH")
		path += ":/home/sigmaos/bin/kernel"
		os.Setenv("PATH", path)
		db.DPrintf(db.BOOT, "Exec spproxy")
		var err error
		spss, err = spproxysrv.ExecSPProxySrv(p, pe.GetInnerContainerIP(), pe.GetOuterContainerIP(), sp.Tpid("NO_PID"))
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error start spproxyd: %v", err)
			return nil, err
		}
		db.DPrintf(db.BOOT, "Exec spproxy done")
	}
	return &DialProxyProvider{
		spkc: spkc,
		spss: spss,
	}, nil
}

func (dpp *DialProxyProvider) Shutdown() error {
	if dpp.spkc != nil {
		defer dpp.spkc.Close()
		if err := dpp.spkc.Shutdown(); err != nil {
			db.DPrintf(db.ALWAYS, "Shutdown spproxyd err %v", err)
			return err
		}
	} else {
		if err := dpp.spss.Evict(); err != nil {
			db.DPrintf(db.ERROR, "Error evict spproxy: %v", err)
			return err
		}
		if err := dpp.spss.Wait(); err != nil {
			db.DPrintf(db.ERROR, "Error wait spproxy: %v", err)
			return err
		}
	}
	return nil
}
