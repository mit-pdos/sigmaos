package system

import (
	"os"
	"os/exec"
	"path"
	"strconv"
	"syscall"

	"sigmaos/bootkernelclnt"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kernelclnt"
	"sigmaos/proc"
	"sigmaos/procclnt"
	sp "sigmaos/sigmap"
)

// Boot ymls
const (
	BOOT_ALL   = "bootall.yml"
	BOOT_NAMED = "boot.yml"
	BOOT_NODE  = "bootmach.yml"
)

type System struct {
	kernels     []*bootkernelclnt.Kernel
	kernelclnts []*kernelclnt.KernelClnt
	nameds      []string
	proxy       *exec.Cmd
}

func bootSystem(realmid, ymldir, ymlname string) (*System, error) {
	sys := &System{}
	sys.kernels = make([]*bootkernelclnt.Kernel, 1)
	sys.kernelclnts = make([]*kernelclnt.KernelClnt, 1)
	db.DPrintf(db.SYSTEM, "Boot system %v %v %v", realmid, ymldir, ymlname)
	k, err := bootkernelclnt.BootKernel(realmid, true, path.Join(ymldir, ymlname))
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.SYSTEM, "Done boot system %v %v %v", realmid, ymldir, ymlname)
	sys.kernels[0] = k
	nameds, err := fslib.SubNamedIP(k.Ip())
	if err != nil {
		return nil, err
	}
	sys.nameds = nameds
	sys.proxy = startProxy(sys.kernels[0].Ip(), nameds)
	if err := sys.proxy.Start(); err != nil {
		return nil, err
	}
	// Make the init kernel clnt
	fsl, _, err := sys.MakeClnt(0, "sys-0")
	if err != nil {
		return nil, err
	}
	sys.kernelclnts[0], err = kernelclnt.MakeKernelClnt(fsl, sp.BOOT+"~local/")
	if err != nil {
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
		// TODO: is the below code necessary?
		//kaddr := []string{""}
		//mnt := sp.MkMountService(kaddr)
		//if err := sys.initkernel.MkMountSymlink(sp.BOOT, mnt); err != nil {
		//	return nil, err
		//}
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
	k, err := bootkernelclnt.BootKernel(realmid, true, path.Join(ymldir, BOOT_NODE))
	if err != nil {
		return err
	}
	sys.kernels = append(sys.kernels, k)
	// Make the init kernel clnt
	idx := len(sys.kernels) - 1
	fsl, _, err := sys.MakeClnt(idx, "sys-"+strconv.Itoa(idx))
	if err != nil {
		return err
	}
	kclnt, err := kernelclnt.MakeKernelClnt(fsl, sp.BOOT+"~local/")
	if err != nil {
		return err
	}
	sys.kernelclnts = append(sys.kernelclnts, kclnt)
	return nil
}

// Make a set of clients (fslib & procclnt) for a specific kernel (with the
// appropriate localip set).
func (sys *System) MakeClnt(kidx int, name string) (*fslib.FsLib, *procclnt.ProcClnt, error) {
	return sys.makeClnt(sys.kernels[kidx].Ip(), name)
}

func (sys *System) makeClnt(kip, name string) (*fslib.FsLib, *procclnt.ProcClnt, error) {
	fsl, err := fslib.MakeFsLibAddr(name, kip, sys.nameds)
	if err != nil {
		return nil, nil, err
	}
	// XXX Should we MakeProcClntInit?
	pclnt := procclnt.MakeProcClntInit(proc.GetPid(), fsl, "test", sys.nameds)
	return fsl, pclnt, nil
}

func (sys *System) GetNamedAddrs() []string {
	return sys.nameds
}

func (sys *System) KillOne(kidx int, sname string) error {
	return sys.kernelclnts[kidx].Kill(sname)
}

func (sys *System) Shutdown() error {
	if err := sys.proxy.Process.Kill(); err != nil {
		return err
	}
	// XXX shut down other kernels first?
	if err := sys.kernels[0].Shutdown(); err != nil {
		return err
	}
	//if err := sys.Root.Shutdown(); err != nil {
	//	return err
	//}
	return nil
}

// XXX make optional?
func startProxy(IP string, nds []string) *exec.Cmd {
	cmd := exec.Command("proxyd", append([]string{IP}, nds...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}
