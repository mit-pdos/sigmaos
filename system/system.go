package system

import (
	"os"
	"os/exec"
	"path"

	"sigmaos/bootkernelclnt"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

// Boot ymls
const (
	BOOT_ALL   = "bootall.yml"
	BOOT_NAMED = "boot.yml"
	BOOT_NODE  = "bootmach.yml"

	NAMEDPORT = ":1111"
)

type System struct {
	kernels []*bootkernelclnt.Kernel
	nameds  sp.Taddrs
	proxy   *exec.Cmd
}

func bootSystem(ymldir, ymlname string) (*System, error) {
	sys := &System{}
	sys.kernels = make([]*bootkernelclnt.Kernel, 1)
	db.DPrintf(db.SYSTEM, "Boot system %v %v", ymldir, ymlname)
	k, nds, err := bootkernelclnt.BootKernelNamed(path.Join(ymldir, ymlname), sp.Taddrs{NAMEDPORT})
	if err != nil {
		return nil, err
	}
	sys.nameds = nds
	db.DPrintf(db.SYSTEM, "Done boot system %v %v %v", ymldir, ymlname, sys.nameds)
	sys.kernels[0] = k
	proc.SetSigmaNamed(sys.nameds)
	sys.proxy = startProxy(sys.kernels[0].GetIP(), sys.nameds)
	if err := sys.proxy.Start(); err != nil {
		return nil, err
	}
	return sys, nil
}

func Boot(n int, ymldir string) (*System, error) {
	sys, err := bootSystem(ymldir, BOOT_ALL)
	if err != nil {
		return nil, err
	}
	for i := 1; i < n; i++ {
		err := sys.BootNode(ymldir)
		if err != nil {
			return nil, err
		}
	}
	return sys, nil
}

func BootNamedOnly(ymldir string) (*System, error) {
	sys, err := bootSystem(ymldir, BOOT_NAMED)
	if err != nil {
		return nil, err
	}
	return sys, nil
}

func (sys *System) BootNode(ymldir string) error {
	k, err := bootkernelclnt.BootKernel(path.Join(ymldir, BOOT_NODE), sys.nameds)
	if err != nil {
		return err
	}
	sys.kernels = append(sys.kernels, k)
	return nil
}

func (sys *System) GetClnt(kidx int) *sigmaclnt.SigmaClnt {
	return sys.kernels[kidx].GetClnt()
}

func (sys *System) GetNamedAddrs() []string {
	return sys.nameds
}

func (sys *System) KillOneK(kidx int, sname string) error {
	return sys.kernels[kidx].KillOne(sname)
}

func (sys *System) KillOne(sname string) error {
	return sys.KillOneK(0, sname)
}

func (sys *System) Boot(s string) error {
	return sys.kernels[0].Boot(s)
}

func (sys *System) BootFss3d() error {
	return sys.kernels[0].Boot(sp.S3REL)
}

func (sys *System) MakeClnt(kidx int, name string) (*sigmaclnt.SigmaClnt, error) {
	return sys.kernels[kidx].MkClnt(name, sys.nameds)
}

func (sys *System) Shutdown() error {
	db.DPrintf(db.SYSTEM, "Shutdown proxyd")
	if err := sys.proxy.Process.Kill(); err != nil {
		return err
	}
	db.DPrintf(db.SYSTEM, "Done shutdown proxyd")
	for i := len(sys.kernels) - 1; i >= 0; i-- {
		db.DPrintf(db.SYSTEM, "Shutdown kernel %v", i)
		// XXX shut down other kernels first?
		if err := sys.kernels[i].Shutdown(); err != nil {
			return err
		}
		db.DPrintf(db.SYSTEM, "Done shutdown kernel %v", i)
	}
	return nil
}

// XXX make optional?
func startProxy(IP string, nds sp.Taddrs) *exec.Cmd {
	cmd := exec.Command("proxyd", append([]string{IP}, nds...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}
