package container

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"

	db "sigmaos/debug"
	"sigmaos/proc"
)

//
// Contain user procs using exec-uproc trampoline
//

func RunUProc(uproc *proc.Proc) error {
	db.DPrintf(db.CONTAINER, "RunUProc %v env %v\n", uproc, os.Environ())
	cmd := exec.Command(uproc.Program, uproc.Args...)
	uproc.AppendEnv("PATH", "/bin:/bin2:/usr/bin")
	cmd.Env = uproc.GetEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Set up new namespaces
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS, // |
		//		syscall.CLONE_NEWUSER,
		//		UidMappings: []syscall.SysProcIDMap{
		//			{
		//				ContainerID: 0, //os.Getuid(),
		//				HostID:      os.Getuid(),
		//				Size:        1,
		//			},
		//		},
		//		GidMappings: []syscall.SysProcIDMap{
		//			{
		//				ContainerID: 0, //os.Getgid(),
		//				HostID:      os.Getgid(),
		//				Size:        1,
		//			},
		//		},
	}
	pn, err := exec.LookPath("exec-uproc")
	if err != nil {
		return fmt.Errorf("RunUProc: LookPath: %v", err)
	}
	cmd.Path = pn
	db.DPrintf(db.CONTAINER, "exec %v\n", cmd)
	defer cleanupJail(uproc.GetPid())
	if err := cmd.Start(); err != nil {
		db.DPrintf(db.CONTAINER, "Error start %v %v", cmd, err)
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	db.DPrintf(db.CONTAINER, "ExecUProc done  %v\n", uproc)
	return nil
}

//
// The exec-uproc trampoline enters here
//

func ExecUProc() error {
	db.DPrintf(db.CONTAINER, "ExecUProc: %v\n", os.Args)
	program := os.Args[0]
	// Isolate the user proc.
	pn, err := isolateUserProc(program)
	if err != nil {
		return err
	}
	db.DPrintf(db.CONTAINER, "exec %v %v", pn, os.Args)
	if err := syscall.Exec(pn, os.Args, os.Environ()); err != nil {
		db.DPrintf(db.CONTAINER, "Error exec %v", err)
		return err
	}
	defer finishIsolation()
	return nil
}

// For debugging
func ls(dir string) error {
	db.DPrintf(db.ALWAYS, "== ls %s\n", dir)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		db.DPrintf(db.ALWAYS, "ls err %v", err)
		return nil
	}
	for _, file := range files {
		db.DPrintf(db.ALWAYS, "file %v isdir %v", file.Name(), file.IsDir())
	}
	return nil
}
