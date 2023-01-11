package system

import (
	"os"
	"os/exec"
	"path"
	"syscall"

	// db "sigmaos/debug"
	"sigmaos/bootkernelclnt"
	"sigmaos/fslib"
	sp "sigmaos/sigmap"
)

const (
	ROOTREALM = "rootrealm"
)

type System struct {
	boot    *bootkernelclnt.Kernel
	kernels []*bootkernelclnt.Kernel
	proxy   *exec.Cmd
}

func Boot(n int, ymldir string) (*System, error) {
	sys := &System{}
	k, err := bootkernelclnt.BootKernel(ROOTREALM, true, path.Join(ymldir, "bootall.yml"))
	if err != nil {
		return nil, err
	}
	sys.boot = k
	nameds, err := fslib.SetNamedIP(k.Ip())
	if err != nil {
		return nil, err
	}
	sys.proxy = startProxy(sys.boot.Ip(), nameds)
	if err := sys.proxy.Start(); err != nil {
		return nil, err
	}
	for i := 1; i < n; i++ {
		_, err := bootkernelclnt.BootKernel(ROOTREALM, true, path.Join(ymldir, "bootmach.yml"))
		if err != nil {
			return nil, err
		}
		kaddr := []string{""}
		mnt := sp.MkMountService(kaddr)
		if err := sys.boot.MkMountSymlink(sp.BOOT, mnt); err != nil {
			return nil, err
		}
	}
	return sys, nil
}

func (sys *System) Shutdown() error {
	if err := sys.boot.Shutdown(); err != nil {
		return err
	}
	//if err := sys.Root.Shutdown(); err != nil {
	//	return err
	//}
	if err := sys.proxy.Process.Kill(); err != nil {
		return err
	}
	return nil
}

func startProxy(IP string, nds []string) *exec.Cmd {
	cmd := exec.Command("proxyd", append([]string{IP}, nds...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}
