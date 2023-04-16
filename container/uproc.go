package container

import (
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
)

//
// Contain user procs using exec-uproc trampoline
//

func RunUProc(uproc *proc.Proc, kernelId string, uprocd proc.Tpid, net string) error {
	db.DPrintf(db.CONTAINER, "RunUProc %v env %v\n", uproc, os.Environ())
	//	cmd := exec.Command("strace", append([]string{"-f", "exec-uproc", uproc.Program}, uproc.Args...)...)
	cmd := exec.Command("exec-uproc", append([]string{uproc.Program}, uproc.Args...)...)
	uproc.AppendEnv("PATH", "/bin:/bin2:/usr/bin:/home/sigmaos/bin/kernel")
	uproc.AppendEnv(proc.SIGMAUPROCD, uprocd.String())
	uproc.AppendEnv(proc.SIGMANET, net)
	cmd.Env = uproc.GetEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Set up new namespaces
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS,
	}
	db.DPrintf(db.CONTAINER, "exec %v\n", cmd)
	defer cleanupJail(uproc.GetPid())
	s := time.Now()
	if err := cmd.Start(); err != nil {
		db.DPrintf(db.CONTAINER, "Error start %v %v", cmd, err)
		return err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] Uproc cmd.Start %v", uproc.GetPid(), time.Since(s))
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
	args := os.Args[1:]
	program := args[0]
	s := time.Now()
	// Isolate the user proc.
	pn, err := isolateUserProc(program)
	db.DPrintf(db.SPAWN_LAT, "[%v] Uproc jail creation %v", proc.GetPid(), time.Since(s))
	if err != nil {
		return err
	}
	db.DPrintf(db.CONTAINER, "exec %v %v", pn, args)
	if err := syscall.Exec(pn, args, os.Environ()); err != nil {
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
