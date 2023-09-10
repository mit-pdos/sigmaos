package bootkernelclnt

import (
	"os/exec"
	"path"

	"sigmaos/proc"
	db "sigmaos/debug"
	"sigmaos/kernelclnt"
	"sigmaos/rand"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

//
// Library to start kernel
//

const (
	START = "../start-kernel.sh"
)

func Start(kernelId string, scfg *proc.ProcEnv, srvs string, overlays bool) (string, error) {
	args := []string{
		"--pull", scfg.BuildTag,
		"--boot", srvs,
		"--named", scfg.EtcdIP,
		"--host",
	}
	if overlays {
		args = append(args, "--overlays")
	}
	args = append(args, kernelId)
	out, err := exec.Command(START, args...).Output()
	if err != nil {
		db.DPrintf(db.BOOT, "Boot: start out %s err %v\n", string(out), err)
		return "", err
	}
	ip := string(out)
	db.DPrintf(db.BOOT, "Start: %v srvs %v IP %v\n", kernelId, srvs, ip)
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

func MkKernelClntStart(scfg *proc.ProcEnv, conf string, overlays bool) (*Kernel, error) {
	kernelId := GenKernelId()
	ip, err := Start(kernelId, scfg, conf, overlays)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.ALWAYS, "Got IP %v", ip)
	return MkKernelClnt(kernelId, scfg)
}

func MkKernelClnt(kernelId string, scfg *proc.ProcEnv) (*Kernel, error) {
	db.DPrintf(db.SYSTEM, "MakeKernelClnt %s\n", kernelId)
	sc, err := sigmaclnt.MkSigmaClntRootInit(scfg)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error make sigma clnt root init")
		return nil, err
	}
	pn := sp.BOOT + kernelId
	if kernelId == "" {
		pn1, _, err := sc.ResolveUnion(sp.BOOT + "~local")
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error resolve local")
			return nil, err
		}
		pn = pn1
		kernelId = path.Base(pn)
	}

	db.DPrintf(db.SYSTEM, "MakeKernelClnt %s %s\n", pn, kernelId)
	kclnt, err := kernelclnt.MakeKernelClnt(sc.FsLib, pn)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error Mkcernelclnt")
		return nil, err
	}

	return &Kernel{sc, kernelId, kclnt}, nil
}

func (k *Kernel) NewSigmaClnt(scfg *proc.ProcEnv) (*sigmaclnt.SigmaClnt, error) {
	return sigmaclnt.MkSigmaClntRootInit(scfg)
}

func (k *Kernel) Shutdown() error {
	db.DPrintf(db.SYSTEM, "Shutdown kernel %s", k.kernelId)
	err := k.kclnt.Shutdown()
	db.DPrintf(db.SYSTEM, "Shutdown kernel %s err %v", k.kernelId, err)
	return err
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
