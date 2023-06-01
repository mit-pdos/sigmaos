package bootkernelclnt

import (
	"errors"
	"os/exec"
	"path"
	"time"

	db "sigmaos/debug"
	"sigmaos/kernelclnt"
	"sigmaos/rand"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

//
// Library to start kernel
//

const (
	START = "../start-kernel.sh"
)

func Start(kernelId, tag, srvs string, namedAddr sp.Taddrs, overlays bool) (string, error) {
	s, e := namedAddr.Taddrs2String()
	if e != nil {
		return "", e
	}
	args := []string{
		"--pull", tag,
		"--boot", srvs,
		"--named", s,
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
	db.DPrintf(db.BOOT, "Start: %v IP %v\n", srvs, ip)
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

func MkKernelClntStart(tag, name, conf string, namedAddr sp.Taddrs, overlays bool) (*Kernel, error) {
	kernelId := GenKernelId()
	ip, err := Start(kernelId, tag, conf, namedAddr, overlays)
	if err != nil {
		return nil, err
	}
	return MkKernelClnt(kernelId, name, ip, namedAddr)
}

func MkKernelClnt(kernelId, name, ip string, namedAddr sp.Taddrs) (*Kernel, error) {
	db.DPrintf(db.SYSTEM, "MakeKernelClnt %s\n", kernelId)
	sc, err := sigmaclnt.MkSigmaClntRootInit(name, ip, namedAddr)
	if err != nil {
		return nil, err
	}
	pn := sp.BOOT + kernelId
	if kernelId == "" {
		pn1, _, err := sc.ResolveUnion(sp.BOOT + "~local")
		if err != nil {
			return nil, err
		}
		pn = pn1
		kernelId = path.Base(pn)
	}

	db.DPrintf(db.SYSTEM, "MakeKernelClnt %s %s\n", pn, kernelId)

	for {
		var serr *serr.Err
		sts, err := sc.GetDir(pn + "/")
		if errors.As(err, &serr) && serr.IsErrNotfound() {
			db.DPrintf(db.SYSTEM, "MakeKernelClnt: try again %s\n", pn)
			time.Sleep(1 * time.Second)
		} else if err != nil {
			db.DPrintf(db.SYSTEM, "MakeKernelClnt %s err %v\n", pn, err)
			return nil, err
		} else {
			db.DPrintf(db.SYSTEM, "MakeKernelClnt %s %v\n", pn, sp.Names(sts))
			break
		}
	}

	kclnt, err := kernelclnt.MakeKernelClnt(sc.FsLib, pn)
	if err != nil {
		return nil, err
	}

	return &Kernel{sc, kernelId, kclnt}, nil
}

func (k *Kernel) MkSigmaClnt(name string) (*sigmaclnt.SigmaClnt, error) {
	return sigmaclnt.MkSigmaClntRootInit(name, k.GetLocalIP(), k.SigmaClnt.NamedAddr())
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
