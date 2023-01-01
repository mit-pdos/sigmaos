package container

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"syscall"
	"time"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

func RunKernelContainer(cmd *exec.Cmd, realm string) error {
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
	if err := mkScnet(cmd.Process.Pid, realm); err != nil {
		return err
	}
	return nil
}

// XXX specialized for one kernel for now
func execKContainer() error {
	rootfs, err := SIGMAROOTFS()
	if err != nil {
		return err
	}

	rand.Seed(time.Now().UnixNano())

	if err := pivotFs(rootfs); err != nil {
		return err
	}

	if err := syscall.Sethostname([]byte("sigmaos")); err != nil {
		return err
	}

	ip := fmt.Sprintf(IPFormat, rand.Intn(253)+2)
	if err := setupScnet(ip); err != nil {
		return err
	}

	if err := syscall.Chdir(sp.SIGMAHOME); err != nil {
		log.Printf("Chdir %s err %v", sp.SIGMAHOME, err)
		return err
	}

	path := os.Getenv("PATH")
	p := sp.SIGMAHOME + "/bin/linux/:" + sp.SIGMAHOME + "/bin/kernel"
	os.Setenv("PATH", path+":"+p)

	db.DPrintf(db.CONTAINER, "env: %v\n", os.Environ())

	pn, err := exec.LookPath(os.Args[1])
	if err != nil {
		return fmt.Errorf("LookPath err %v", err)
	}

	db.DPrintf(db.CONTAINER, "exec %s %v\n", pn, os.Args[1:])
	return syscall.Exec(pn, os.Args[1:], os.Environ())
}
