package container

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"syscall"

	selinux "github.com/opencontainers/selinux/go-selinux"

	db "sigmaos/debug"
	"sigmaos/perf"
	"sigmaos/seccomp"
	sp "sigmaos/sigmap"
)

func isolateUserProc(program string) (string, error) {
	// Read the seccomp whitelist before doing the pivot_root.
	sigmaSeccomp, err := seccomp.ReadWhiteList("seccomp/whitelist.yml")
	if err != nil {
		return "", err
	}
	// Setup and chroot to the process jail.
	if err := jailProcess(); err != nil {
		db.DPrintf(db.CONTAINER, "Error jail process %v", err)
		return "", err
	}
	pn, err := exec.LookPath(program)
	if err != nil {
		return "", fmt.Errorf("ContainUProc: LookPath: %v", err)
	}
	// Load the sigmaOS seccomp white list.
	seccomp.LoadFilter(sigmaSeccomp)
	db.DPrintf(db.CONTAINER, "Apply sigma whitelist %v %v", os.Args, sigmaSeccomp)
	// Lock the OS thread, since SE Linux labels are per-thread, and so this
	// thread should disallow the Go runtime from scheduling it on another kernel
	// thread before starting the user proc.
	runtime.LockOSThread()
	// Apply SELinux Label
	if selinux.GetEnabled() {
		plabel, flabel := selinux.InitContainerLabels()
		if err := selinux.SetExecLabel(plabel); err != nil {
			return "", err
		}
		if err := selinux.SetFileLabel(pn, flabel); err != nil {
			return "", err
		}
	}
	return pn, nil
}

func finishIsolation() {
	runtime.UnlockOSThread()
}

// XXX pair down what is being mounted; exec needs a lot, but maybe
// not all of it (e.g., usr? and only some subdirectories)
func jailProcess() error {
	newRoot := path.Join(sp.SIGMAHOME, "jail")
	// Create directories to use as mount points, as well as the new root directory itself.
	for _, d := range []string{"", OLD_ROOT_MNT, "lib", "usr", "lib64", "etc", "sys", "dev", "proc", "bin", "bin2", "tmp", perf.OUTPUT_PATH} {
		if err := os.Mkdir(path.Join(newRoot, d), 0700); err != nil {
			db.DPrintf(db.ALWAYS, "failed to mkdir [%v]: %v", d, err)
			return err
		}
	}
	// Mount new file system as a mount point so we can pivot_root to it later
	if err := syscall.Mount(newRoot, newRoot, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount new root filesystem: %v", err)
		return err
	}
	// Chdir to new root
	if err := syscall.Chdir(newRoot); err != nil {
		db.DPrintf(db.ALWAYS, "failed to chdir to /: %v", err)
		return err
	}
	// Mount realm's user bin directory as /bin
	if err := syscall.Mount(path.Join(sp.SIGMAHOME, "bin/user"), "bin", "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount userbin: %v", err)
		return err
	}
	// Mount realm's kernel bin directory as /bin2
	if err := syscall.Mount(path.Join(sp.SIGMAHOME, "bin/kernel"), "bin2", "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount kernelbin: %v", err)
		return err
	}
	// Mount /lib
	if err := syscall.Mount("/lib", "lib", "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount /lib: %v", err)
		return err
	}
	// Mount /lib64
	if err := syscall.Mount("/lib64", "lib64", "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount /lib64: %v", err)
		return err
	}
	// Mount /proc
	if err := syscall.Mount("proc", "proc", "proc", 0, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount /proc: %v", err)
		return err
	}
	// Mount perf dir (remove starting first slash)
	if err := syscall.Mount(perf.OUTPUT_PATH, perf.OUTPUT_PATH[1:], "none", syscall.MS_BIND, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount perfoutput: %v", err)
		return err
	}
	// Mount /usr
	if err := syscall.Mount("/usr", "usr", "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount /usr: %v", err)
		return err
	}
	// Mount /sys for /sys/devices/system/cpu/online; XXX exclude
	// /sys/firmware; others?
	if err := syscall.Mount("/sys", "sys", "sysfs", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount /sys err %v", err)
		return err
	}
	// Mount /dev/urandom
	if err := syscall.Mount("/dev", "dev", "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount /dev: %v", err)
		return err
	}
	// Mount /etc
	if err := syscall.Mount("/etc", "etc", "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount /etc: %v", err)
		return err
	}
	// ========== No more mounts beyond this point ==========
	// pivot_root
	if err := syscall.PivotRoot(".", OLD_ROOT_MNT); err != nil {
		db.DPrintf(db.ALWAYS, "failed to pivot root: %v", err)
		return err
	}
	// Chdir to new root
	if err := syscall.Chdir("/"); err != nil {
		db.DPrintf(db.ALWAYS, "failed to chdir to /: %v", err)
		return err
	}
	// unmount the old root filesystem
	if err := syscall.Unmount(OLD_ROOT_MNT, syscall.MNT_DETACH); err != nil {
		db.DPrintf(db.ALWAYS, "failed to unmount the old root filesystem: %v", err)
		return err
	}
	// Remove the old root filesystem
	if err := os.Remove(OLD_ROOT_MNT); err != nil {
		db.DPrintf(db.ALWAYS, "failed to remove old root filesystem: %v", err)
		return err
	}
	db.DPrintf(db.CONTAINER, "Successfully pivoted to new root %v", newRoot)
	return nil
}
