package system

import (
	"os"
	"os/exec"
	"path"

	"sigmaos/bootkernelclnt"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/procclnt"
)

// Boot ymls
const (
	BOOT_ALL   = "bootall.yml"
	BOOT_NAMED = "boot.yml"
	BOOT_NODE  = "bootmach.yml"
)

type System struct {
	kernels []*bootkernelclnt.Kernel
	nameds  []string
	proxy   *exec.Cmd
}

func bootSystem(realmid, ymldir, ymlname string) (*System, error) {
	sys := &System{}
	sys.kernels = make([]*bootkernelclnt.Kernel, 1)
	db.DPrintf(db.SYSTEM, "Boot system %v %v %v", realmid, ymldir, ymlname)
	k, nds, err := bootkernelclnt.BootKernelNamed(path.Join(ymldir, ymlname), []string{":1111"})
	if err != nil {
		return nil, err
	}
	sys.nameds = nds
	db.DPrintf(db.SYSTEM, "Done boot system %v %v %v %v", realmid, ymldir, ymlname, sys.nameds)
	sys.kernels[0] = k
	fslib.SetSigmaNamed(sys.nameds)
	sys.proxy = startProxy(sys.kernels[0].GetIP(), sys.nameds)
	if err := sys.proxy.Start(); err != nil {
		return nil, err
	}
	return sys, nil
}

func Boot(realmid string, n int, ymldir string) (*System, error) {
	sys, err := bootSystem(realmid, ymldir, BOOT_ALL)
	if err != nil {
		return nil, err
	}
	for i := 1; i < n; i++ {
		err := sys.BootNode(realmid, ymldir)
		if err != nil {
			return nil, err
		}
	}
	return sys, nil
}

func BootNamedOnly(realmid, ymldir string) (*System, error) {
	sys, err := bootSystem(realmid, ymldir, BOOT_NAMED)
	if err != nil {
		return nil, err
	}
	return sys, nil
}

func (sys *System) BootNode(realmid, ymldir string) error {
	k, err := bootkernelclnt.BootKernel(path.Join(ymldir, BOOT_NODE), sys.nameds)
	if err != nil {
		return err
	}
	sys.kernels = append(sys.kernels, k)
	return nil
}

func (sys *System) GetClnt(kidx int) (*fslib.FsLib, *procclnt.ProcClnt) {
	return sys.kernels[kidx].GetClnt()
}

func (sys *System) GetNamedAddrs() []string {
	return sys.nameds
}

func (sys *System) KillOne(kidx int, sname string) error {
	return sys.kernels[kidx].KillOne(sname)
}

func (sys *System) MakeClnt(kidx int, name string) (*fslib.FsLib, *procclnt.ProcClnt, error) {
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
func startProxy(IP string, nds []string) *exec.Cmd {
	cmd := exec.Command("proxyd", append([]string{IP}, nds...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}
