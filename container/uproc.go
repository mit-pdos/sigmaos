package container

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"syscall"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/seccomp"
	sp "sigmaos/sigmap"
)

//
// Contain user procs using exec-uproc trampoline
//

func RunUProc(uproc *proc.Proc) error {
	db.DPrintf(db.CONTAINER, "RunUProc %v env %v\n", uproc, os.Environ())

	cmd := exec.Command(uproc.Program, uproc.Args...)
	uproc.AppendEnv("PATH", "/home/sigmaos/bin/user:/bin/:/usr/bin")
	cmd.Env = uproc.GetEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// // Set up new namespaces
	// cmd.SysProcAttr = &syscall.SysProcAttr{
	// 	Cloneflags: syscall.CLONE_NEWUTS |
	// 		syscall.CLONE_NEWNS |
	// 		syscall.CLONE_NEWIPC |
	// 		syscall.CLONE_NEWPID |
	// 		syscall.CLONE_NEWUSER,
	// 	UidMappings: []syscall.SysProcIDMap{
	// 		{
	// 			ContainerID: 0,
	// 			HostID:      os.Getuid(),
	// 			Size:        1,
	// 		},
	// 	},
	// 	GidMappings: []syscall.SysProcIDMap{
	// 		{
	// 			ContainerID: 0,
	// 			HostID:      os.Getgid(),
	// 			Size:        1,
	// 		},
	// 	},
	// }

	pn, err := exec.LookPath("exec-uproc")
	if err != nil {
		return fmt.Errorf("RunUProc: LookPath: %v", err)
	}
	cmd.Path = pn
	db.DPrintf(db.CONTAINER, "exec %v\n", cmd)

	if err := cmd.Start(); err != nil {
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

	wl, err := seccomp.ReadWhiteList("seccomp/whitelist.yml")
	if err != nil {
		return err
	}
	seccomp.LoadFilter(wl)

	pn, err := exec.LookPath(os.Args[0])
	if err != nil {
		return fmt.Errorf("ContainUProc: LookPath: %v", err)
	}
	db.DPrintf(db.CONTAINER, "exec %v %v %v\n", pn, os.Args, wl)
	return syscall.Exec(pn, os.Args, os.Environ())
}

// For debugging
func ls(dir string) error {
	log.Printf("== ls %s\n", dir)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, file := range files {
		log.Println(file.Name(), file.IsDir())
	}
	return nil
}

// XXX pair down what is being mounted; exec needs a lot, but maybe
// not all of it (e.g., usr? and only some subdirectories)
func setupFs(newRoot string) error {
	oldRootMnt := "old_root" + rand.String(8)

	// xnewRoot := newRoot
	newRoot = newRoot + "/" + string(sp.ROOTREALM)

	log.Printf("new root %v\n", newRoot)

	// Mount new file system as a mount point so we can pivot_root to it later
	if err := syscall.Mount(newRoot, newRoot, "", syscall.MS_BIND, ""); err != nil {
		log.Printf("failed to mount new root filesystem: %v", err)
		return err
	}

	// Chdir to new root
	if err := syscall.Chdir(newRoot); err != nil {
		log.Printf("failed to chdir to /: %v", err)
		return err
	}

	// Make dir for oldMount
	if err := syscall.Mkdir(oldRootMnt, 0700); err != nil {
		log.Printf("failed to mkdir: %v", err)
		return err
	}

	ls(".")

	// // Mount /sys for /sys/devices/system/cpu/online; XXX exclude
	// // /sys/firmware; others?
	// if err := syscall.Mount("/sys", path.Join(newRoot, "sys"), "sysfs", syscall.MS_BIND, ""); err != nil {
	// 	log.Printf("failed to mount /sys err %v", err)
	// 	return err
	// }

	// // Mount /dev/urandom
	// if err := syscall.Mount("/dev", "dev", "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
	// 	log.Printf("failed to mount /dev: %v", err)
	// 	return err
	// }

	// Mount /usr
	if err := syscall.Mount("/usr", "usr", "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		log.Printf("failed to mount /dev/usr: %v", err)
		return err
	}

	// Mount /lib
	if err := syscall.Mount("/lib", "lib", "none", syscall.MS_BIND, ""); err != nil {
		log.Printf("failed to mount /dev/lib: %v", err)
		return err
	}

	// Mount /lib
	if err := syscall.Mount("/lib64", "lib64", "none", syscall.MS_BIND, ""); err != nil {
		log.Printf("failed to mount /dev/lib64: %v", err)
		return err
	}

	// Mount /etc
	if err := syscall.Mount("/etc", "etc", "none", syscall.MS_BIND, ""); err != nil {
		log.Printf("failed to mount /etc: %v", err)
		return err
	}

	// // Mount bin/user on /bin so that user procs can run only programs from /bin/user
	// if err := syscall.Mount(path.Join(xnewRoot)+"/bin/user", path.Join(newRoot, UBIN), "none", syscall.MS_BIND, ""); err != nil {
	// 	log.Printf("failed to mount /bin: %v", err)
	// 	return err
	// }

	// pivot_root
	if err := syscall.PivotRoot(".", oldRootMnt); err != nil {
		log.Printf("failed to pivot root: %v", err)
		return err
	}

	// Chdir to new root
	if err := syscall.Chdir("/"); err != nil {
		log.Printf("failed to chdir to /: %v", err)
		return err
	}

	// Mount proc
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		log.Printf("failed to mount /proc: %v", err)
		return err
	}

	// unmount the old root filesystem
	if err := syscall.Unmount(oldRootMnt, syscall.MNT_DETACH); err != nil {
		log.Printf("failed to unmount the old root filesystem: %v", err)
		return err
	}

	// Remove the old root filesystem
	if err := os.Remove(oldRootMnt); err != nil {
		log.Printf("failed to remove old root filesystem: %v", err)
		return err
	}

	return nil
}
