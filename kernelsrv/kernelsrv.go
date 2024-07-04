package kernelsrv

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/kernel"
	"sigmaos/kernelsrv/proto"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

type KernelSrv struct {
	k  *kernel.Kernel
	ch chan struct{}
}

func RunKernelSrv(k *kernel.Kernel) error {
	ks := &KernelSrv{k: k}
	ks.ch = make(chan struct{})
	db.DPrintf(db.KERNEL, "Run KernelSrv %v", k.Param.KernelID)
	sc := sigmaclnt.NewSigmaClntProcAPI(k.SigmaClntKernel)
	_, err := sigmasrv.NewSigmaSrvClnt(sp.BOOT+k.Param.KernelID, sc, ks)
	if err != nil {
		return err
	}
	// let start-kernel.sh know that the kernel is ready
	f, err := os.Create("/tmp/sigmaos/" + k.Param.KernelID)
	if err != nil {
		return err
	}
	f.Close()
	<-ks.ch
	db.DPrintf(db.KERNEL, "Run KernelSrv done %v", k.Param.KernelID)
	return nil
}

func (ks *KernelSrv) Boot(ctx fs.CtxI, req proto.BootRequest, rep *proto.BootResult) error {
	db.DPrintf(db.KERNEL, "kernelsrv boot %v args %v", req.Name, req.Args)
	var pid sp.Tpid
	var err error
	if pid, err = ks.k.BootSub(req.Name, req.Args, ks.k.Param, sp.Trealm(req.RealmStr)); err != nil {
		return err
	}
	rep.PidStr = pid.String()
	db.DPrintf(db.KERNEL, "kernelsrv boot done %v pid %v", req.Name, pid)
	return nil
}

func (ks *KernelSrv) SetCPUShares(ctx fs.CtxI, req proto.SetCPUSharesRequest, rep *proto.SetCPUSharesResponse) error {
	return ks.k.SetCPUShares(sp.Tpid(req.PidStr), req.Shares)
}

func (ks *KernelSrv) GetCPUUtil(ctx fs.CtxI, req proto.GetKernelSrvCPUUtilRequest, rep *proto.GetKernelSrvCPUUtilResponse) error {
	util, err := ks.k.GetCPUUtil(sp.Tpid(req.PidStr))
	if err != nil {
		return err
	}
	rep.Util = util
	return nil
}

func (ks *KernelSrv) EvictKernelProc(ctx fs.CtxI, req proto.EvictKernelProcRequest, rep *proto.EvictKernelProcResponse) error {
	return ks.k.EvictKernelProc(sp.Tpid(req.PidStr))
}

func (ks *KernelSrv) Shutdown(ctx fs.CtxI, req proto.ShutdownRequest, rep *proto.ShutdownResult) error {
	db.DPrintf(db.KERNEL, "%v: kernelsrv begin shutdown (spproxyd %t)", ks.k.Param.KernelID, ks.k.IsPurelySPProxydKernel())
	if ks.k.IsPurelySPProxydKernel() {
		// This is the last container to shut down, so no named isn't up anymore.
		// Normal shutdown would involve ending leases, etc., which takes a long
		// time. Instead, shortcut this by killing spproxyd and just exiting.
		db.DPrintf(db.KERNEL, "Shutdown spproxyd kernelsrv")
	} else {
		if err := ks.k.Remove(sp.BOOT + ks.k.Param.KernelID); err != nil {
			db.DPrintf(db.KERNEL, "%v: kernelsrv shutdown remove err %v", ks.k.Param.KernelID, err)
		}
	}
	if err := ks.k.Shutdown(); err != nil {
		return err
	}
	db.DPrintf(db.KERNEL, "%v: kernelsrv done shutdown", ks.k.Param.KernelID)
	ks.ch <- struct{}{}
	return nil
}

func (ks *KernelSrv) Kill(ctx fs.CtxI, req proto.KillRequest, rep *proto.KillResult) error {
	return ks.k.KillOne(req.Name)
}
