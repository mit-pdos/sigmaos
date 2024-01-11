// Package bootkernelclnt starts a SigmaOS kernel
package bootkernelclnt

import (
	"os/exec"
	"path"

	db "sigmaos/debug"
	"sigmaos/kernelclnt"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

// Shell script that starts the sigmaos container, which invokes Start
// of [bootkernel]
const (
	START = "../start-kernel.sh"
)

func Start(kernelId string, pcfg *proc.ProcEnv, srvs string, overlays bool, provider sp.Tprovider, gvisor bool) (string, error) {
	args := []string{
		"--pull", pcfg.BuildTag,
		"--boot", srvs,
		"--named", pcfg.EtcdIP,
		"--host",
	}
	if overlays {
		args = append(args, "--overlays")
	}
	if gvisor {
		args = append(args, "--gvisor")
	}
	args = append(args, []string{"--provider", provider.String()}...)
	args = append(args, kernelId)
	out, err := exec.Command(START, args...).Output()
	if err != nil {
		db.DPrintf(db.BOOT, "Boot: start out %s err %v\n", string(out), err)
		return "", err
	}
	ip := string(out)
	db.DPrintf(db.BOOT, "Start: %v srvs %v IP %v overlays %v gvisor %v", kernelId, srvs, ip, overlays, gvisor)
	return ip, nil
}

func GenKernelId() string {
	return "sigma-" + rand.String(4)
}

type Kernel struct {
	*sigmaclnt.SigmaClnt
	kernelId string
	kclnt    *kernelclnt.KernelClnt
}

func NewKernelClntStart(pcfg *proc.ProcEnv, conf string, overlays bool, provider sp.Tprovider, gvisor bool) (*Kernel, error) {
	kernelId := GenKernelId()
	_, err := Start(kernelId, pcfg, conf, overlays, provider, gvisor)
	if err != nil {
		return nil, err
	}
	return NewKernelClnt(kernelId, pcfg)
}

func NewKernelClnt(kernelId string, pcfg *proc.ProcEnv) (*Kernel, error) {
	db.DPrintf(db.SYSTEM, "NewKernelClnt %s\n", kernelId)
	sc, err := sigmaclnt.NewSigmaClntRootInit(pcfg)
	if err != nil {
		db.DPrintf(db.ALWAYS, "NewKernelClnt sigmaclnt err %v", err)
		return nil, err
	}
	pn := sp.BOOT + kernelId
	if kernelId == "" {
		var pn1 string
		var err error
		if pcfg.EtcdIP != pcfg.GetLocalIP().String() {
			// If running in a distributed setting, bootkernel clnt can be ~any
			pn1, _, err = sc.ResolveMount(sp.BOOT + "~any")
		} else {
			pn1, _, err = sc.ResolveMount(sp.BOOT + "~local")
		}
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error resolve local")
			return nil, err
		}
		pn = pn1
		kernelId = path.Base(pn)
	}

	db.DPrintf(db.SYSTEM, "NewKernelClnt %s %s\n", pn, kernelId)
	kclnt, err := kernelclnt.NewKernelClnt(sc.FsLib, pn)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error NewKernelClnt %v", err)
		return nil, err
	}

	return &Kernel{sc, kernelId, kclnt}, nil
}

func (k *Kernel) NewSigmaClnt(pcfg *proc.ProcEnv) (*sigmaclnt.SigmaClnt, error) {
	return sigmaclnt.NewSigmaClntRootInit(pcfg)
}

func (k *Kernel) Shutdown() error {
	db.DPrintf(db.SYSTEM, "Shutdown kernel %s", k.kernelId)
	err := k.kclnt.Shutdown()
	db.DPrintf(db.SYSTEM, "Shutdown kernel %s err %v", k.kernelId, err)
	if err != nil {
		return err
	}
	return nil
}

func (k *Kernel) Boot(s string) error {
	_, err := k.kclnt.Boot(s, []string{})
	return err
}

func (k *Kernel) Kill(s string) error {
	return k.kclnt.Kill(s)
}

func (k *Kernel) KernelId() string {
	return k.kernelId
}
