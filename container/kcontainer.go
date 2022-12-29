package container

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"syscall"
	"time"

	db "sigmaos/debug"
)

func RunKernelContainer(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}
	cmd.Args = append([]string{KERNEL}, cmd.Args...)

	pn, err := exec.LookPath("exec-container")
	if err != nil {
		return fmt.Errorf("LookPath: %v", err)
	}

	cmd.Path = pn

	db.DPrintf(db.CONTAINER, "contain cmd  %v\n", cmd)

	if err := cmd.Start(); err != nil {
		return err
	}

	db.DPrintf(db.CONTAINER, "mkscnet %v\n", cmd.Process.Pid)
	if err := mkScnet(cmd.Process.Pid); err != nil {
		return err
	}
	return nil
}

// XXX specialized for one kernel for now
func setupKContainer(rootfs string) error {
	db.DPrintf(db.CONTAINER, "execContainer: %v %s\n", os.Args, rootfs)

	rand.Seed(time.Now().UnixNano())

	if err := setupFs(rootfs); err != nil {
		return err
	}

	if err := syscall.Sethostname([]byte("sigmaos")); err != nil {
		return err
	}

	ip := fmt.Sprintf(IPFormat, rand.Intn(253)+2, rand.Intn(253)+2)
	if err := setupScnet(ip); err != nil {
		return err
	}
	return nil
}
