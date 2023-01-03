package container

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"

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

	ip, rip := mkIpNet()
	cmd.Args = append([]string{KERNEL, ip, realm}, cmd.Args...)

	pn, err := exec.LookPath("exec-container")
	if err != nil {
		return fmt.Errorf("LookPath: %v", err)
	}

	cmd.Path = pn

	db.DPrintf(db.CONTAINER, "Contain kernel cmd  %v\n", cmd)

	if err := cmd.Start(); err != nil {
		return err
	}

	db.DPrintf(db.CONTAINER, "mkscnet %v %s %s\n", cmd.Process.Pid, rip, realm)
	if err := mkScnet(cmd.Process.Pid, rip, realm); err != nil {
		return err
	}
	return nil
}

func execKContainer() error {
	rootfs, err := SIGMAROOTFS()
	if err != nil {
		return err
	}

	if err := pivotFs(rootfs); err != nil {
		return err
	}

	realm := os.Args[2]
	dir := sp.SIGMAHOME + "/" + realm
	if err := syscall.Sethostname([]byte(realm)); err != nil {
		return err
	}

	if err := setupScnet(os.Args[1]); err != nil {
		return err
	}

	if err := syscall.Chdir(dir); err != nil {
		log.Printf("Chdir %s err %v", sp.SIGMAHOME, err)
		return err
	}

	path := os.Getenv("PATH")
	p := dir + "/bin/linux/:" + dir + "/bin/kernel"
	os.Setenv("PATH", path+":"+p)

	db.DPrintf(db.CONTAINER, "env: %v\n", os.Environ())

	pn, err := exec.LookPath(os.Args[3])
	if err != nil {
		return fmt.Errorf("LookPath err %v", err)
	}

	db.DPrintf(db.CONTAINER, "exec %s %v\n", pn, os.Args[3:])
	return syscall.Exec(pn, os.Args[3:], os.Environ())
}
