// Package bootkernelclnt starts a SigmaOS kernel
package bootkernelclnt

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	db "sigmaos/debug"
	kernelclnt "sigmaos/kernel/clnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/rand"
)

// Shell script that starts the sigmaos container, which invokes Start
// of [bootkernel]
const (
	START     = "start-kernel.sh"
	K_OUT_DIR = "/tmp/sigmaos-kernel-start-logs"
)

func projectRootPath() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(b))
}

func Start(kernelId string, etcdIP sp.Tip, pe *proc.ProcEnv, ntype Tboot, dialproxy bool, homeDir string, projectRoot string, net string) (string, error) {
	args := []string{
		"--pull", pe.BuildTag,
		"--boot", ntype.String(),
		"--named", etcdIP.String(),
		"--net", net,
	}
	if homeDir != "" {
		args = append(args, "--homedir")
		args = append(args, homeDir)
	}
	if projectRoot != "" {
		args = append(args, "--projectroot")
		args = append(args, projectRoot)
	}
	if dialproxy {
		args = append(args, "--usedialproxy")
	}
	args = append(args, kernelId)
	// Ensure the kernel output directory has been created
	os.Mkdir(K_OUT_DIR, 0777)
	ofilePath := filepath.Join(K_OUT_DIR, kernelId+".stdout")
	ofile, err := os.Create(ofilePath)
	if err != nil {
		db.DPrintf(db.ERROR, "Create out file %v", ofilePath)
		return "", err
	}
	defer ofile.Close()
	efilePath := filepath.Join(K_OUT_DIR, kernelId+".stderr")
	efile, err := os.Create(efilePath)
	if err != nil {
		db.DPrintf(db.ERROR, "Create out file %v", ofilePath)
		return "", err
	}
	defer efile.Close()
	// Create the command struct and set stdout/stderr
	cmd := exec.Command(filepath.Join(projectRootPath(), START), args...)
	cmd.Stdout = ofile
	cmd.Stderr = efile
	if err := cmd.Run(); err != nil {
		db.DPrintf(db.BOOT, "Boot: run err %v", err)
		return "", err
	}
	if err := ofile.Sync(); err != nil {
		db.DPrintf(db.ERROR, "Sync out file %v: %v", ofilePath, err)
		return "", err
	}
	out, err := os.ReadFile(ofilePath)
	if err != nil {
		db.DPrintf(db.ERROR, "Read out file %v: %v", ofilePath, err)
		return "", err
	}
	ip := string(out)
	db.DPrintf(db.BOOT, "Start: %v nodetype %v IP %v dialproxy %v", kernelId, ntype, ip, dialproxy)
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

func NewKernelClntStart(etcdIP sp.Tip, pe *proc.ProcEnv, ntype Tboot, dialproxy bool, homeDir string, projectRoot string, net string) (*Kernel, error) {
	kernelId := GenKernelId()
	// XXX
	_, err := Start(kernelId, etcdIP, pe, ntype, dialproxy, homeDir, projectRoot, net)
	if err != nil {
		return nil, err
	}
	return NewKernelClnt(kernelId, etcdIP, pe)
}

func NewKernelClnt(kernelId string, etcdIP sp.Tip, pe *proc.ProcEnv) (*Kernel, error) {
	db.DPrintf(db.KERNEL, "NewKernelClnt %s\n", kernelId)
	sc, err := sigmaclnt.NewSigmaClntRootInit(pe)
	if err != nil {
		db.DPrintf(db.ALWAYS, "NewKernelClnt sigmaclnt err %v", err)
		return nil, err
	}
	pn := sp.BOOT + kernelId
	if kernelId == "" {
		var pn1 string
		var err error
		if etcdIP != pe.GetOuterContainerIP() {
			// If running in a distributed setting, bootkernel clnt can be ANY
			pn1, _, err = sc.ResolveMount(sp.BOOT + sp.ANY)
		} else {
			pn1, _, err = sc.ResolveMount(sp.BOOT + sp.LOCAL)
		}
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error resolve local")
			return nil, err
		}
		pn = pn1
		kernelId = filepath.Base(pn)
	}

	db.DPrintf(db.KERNEL, "NewKernelClnt %s %s\n", pn, kernelId)
	kclnt, err := kernelclnt.NewKernelClnt(sc.FsLib, pn)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error NewKernelClnt %v", err)
		return nil, err
	}
	return &Kernel{sc, kernelId, kclnt}, nil
}

func (k *Kernel) NewSigmaClnt(pe *proc.ProcEnv) (*sigmaclnt.SigmaClnt, error) {
	return sigmaclnt.NewSigmaClntRootInit(pe)
}

func (k *Kernel) Shutdown() error {
	db.DPrintf(db.KERNEL, "Shutdown kernel %s", k.kernelId)
	k.SigmaClnt.StopWatchingSrvs()
	db.DPrintf(db.KERNEL, "Stopped watching kernelclnt %v", k.kernelId)
	err := k.kclnt.Shutdown()
	db.DPrintf(db.KERNEL, "Shutdown kernel %s err %v", k.kernelId, err)
	if err != nil {
		return err
	}
	return nil
}

func (k *Kernel) Boot(s string) error {
	_, err := k.kclnt.Boot(s, []string{}, []string{})
	return err
}

func (k *Kernel) BootEnv(s string, env []string) error {
	_, err := k.kclnt.Boot(s, []string{}, env)
	return err
}

func (k *Kernel) Kill(s string) error {
	return k.kclnt.Kill(s)
}

func (k *Kernel) KernelId() string {
	return k.kernelId
}
