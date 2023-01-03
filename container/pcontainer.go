package container

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"syscall"

	db "sigmaos/debug"
	"sigmaos/rand"
	"sigmaos/seccomp"
	sp "sigmaos/sigmap"
)

func MakeProcContainer(cmd *exec.Cmd, realmid string) error {
	// Set up new namespaces
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWIPC |
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
	pn, err := exec.LookPath("exec-container")
	if err != nil {
		return fmt.Errorf("LookPath: %v", err)
	}
	cmd.Path = pn
	cmd.Args = append([]string{PROC, realmid}, cmd.Args...)

	db.DPrintf(db.CONTAINER, "Contain proc cmd %v %v\n", cmd, os.Environ())
	return nil
}

func execPContainer() error {
	wl, err := seccomp.ReadWhiteList("./whitelist.yml")
	if err != nil {
		return err
	}

	db.DPrintf(db.CONTAINER, "wl %v env: %v\n", wl, os.Environ())

	if err := setupFs(path.Join(sp.SIGMAHOME, os.Args[1])); err != nil {
		return err
	}

	seccomp.LoadFilter(wl)

	pn, err := exec.LookPath(os.Args[2])
	if err != nil {
		return fmt.Errorf("LookPath err %v", err)
	}

	db.DPrintf(db.CONTAINER, "exec %s %v\n", pn, os.Args[2:])
	return syscall.Exec(pn, os.Args[2:], os.Environ())
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

	// Mount new file system as a mount point so we can pivot_root to it later
	if err := syscall.Mount(newRoot, newRoot, "", syscall.MS_BIND, ""); err != nil {
		log.Printf("failed to mount new root filesystem: %v", err)
		return err
	}

	// Mount old file system
	if err := syscall.Mkdir(path.Join(newRoot, oldRootMnt), 0700); err != nil {
		log.Printf("failed to mkdir: %v", err)
		return err
	}

	// Mount /sys for /sys/devices/system/cpu/online; XXX exclude
	// /sys/firmware; others?
	if err := syscall.Mount("/sys", path.Join(newRoot, "sys"), "sysfs", syscall.MS_BIND, ""); err != nil {
		log.Printf("failed to mount /sys err %v", err)
		return err
	}

	// Mount /dev/urandom
	if err := syscall.Mount("/dev/urandom", path.Join(newRoot, "dev/urandom"), "none", syscall.MS_BIND, ""); err != nil {
		log.Printf("failed to mount /dev/urandom: %v", err)
		return err
	}

	// Mount /usr
	if err := syscall.Mount("/usr", path.Join(newRoot, "usr"), "none", syscall.MS_BIND, ""); err != nil {
		log.Printf("failed to mount /dev/usr: %v", err)
		return err
	}

	// Mount /lib
	if err := syscall.Mount("/lib", path.Join(newRoot, "lib"), "none", syscall.MS_BIND, ""); err != nil {
		log.Printf("failed to mount /dev/lib: %v", err)
		return err
	}

	// Mount /lib
	if err := syscall.Mount("/lib64", path.Join(newRoot, "lib64"), "none", syscall.MS_BIND, ""); err != nil {
		log.Printf("failed to mount /dev/lib64: %v", err)
		return err
	}

	// Mount /etc
	if err := syscall.Mount("/etc", path.Join(newRoot, "etc"), "none", syscall.MS_BIND, ""); err != nil {
		log.Printf("failed to mount /etc: %v", err)
		return err
	}

	// Mount /bin
	if err := syscall.Mount(path.Join(newRoot)+"/bin/user", path.Join(newRoot, "/bin"), "none", syscall.MS_BIND, ""); err != nil {
		log.Printf("failed to mount /bin: %v", err)
		return err
	}

	// pivot_root
	if err := syscall.PivotRoot(newRoot, path.Join(newRoot, oldRootMnt)); err != nil {
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
