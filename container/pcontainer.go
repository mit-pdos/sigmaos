package container

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime/debug"
	"syscall"

	db "sigmaos/debug"
	"sigmaos/seccomp"
	// "sigmaos/proc"
	sp "sigmaos/sigmap"
)

const PRIVILEGED_BIN = sp.SIGMAHOME + "/bin"

func MakeProcContainer(cmd *exec.Cmd) {
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
	cmd.Args = append([]string{PROC}, cmd.Args...)
	cmd.Path = path.Join(PRIVILEGED_BIN, "linux/exec-container")
	db.DPrintf(db.CONTAINER, "contain cmd  %v\n", cmd)
}

func execPContainer() error {
	wl, err := seccomp.ReadWhiteList("./whitelist.yml")
	if err != nil {
		return err
	}
	db.DPrintf(db.CONTAINER, "wl %v env: %v\n", wl, os.Environ())

	if err := setupFs(sp.SIGMAHOME); err != nil {
		return err
	}

	seccomp.LoadFilter(wl)

	pn, err := exec.LookPath(os.Args[1])
	if err != nil {
		return fmt.Errorf("LookPath err %v", err)
	}

	db.DPrintf(db.CONTAINER, "exec %s %v\n", pn, os.Args[1:])
	return syscall.Exec(pn, os.Args[1:], os.Environ())
}

func Isolate(fsRoot string) error {
	if err := createFSNamespace(fsRoot); err != nil {
		log.Printf("Error CreateFSNamespace in namespace.Isolate [%v]: %v", fsRoot, err)
		return err
	}
	if err := isolateFSNamespace(fsRoot); err != nil {
		log.Printf("Error IsolateFSNamespace in namespace.Isolate [%v]: %v", fsRoot, err)
		return err
	}
	return nil
}

func Destroy(fsRoot string) error {
	if err := destroyFSNamespace(fsRoot); err != nil {
		return err
	}
	return nil
}

func createFSNamespace(root string) error {
	dirs := []string{
		"proc",
		"dev",
	}
	if err := os.Mkdir(root, 0777); err != nil {
		debug.PrintStack()
		log.Printf("Error Mkdir1 %v in namespace.CreateFSNamespace: %v", root, err)
		return err
	}
	for _, dname := range dirs {
		if err := os.Mkdir(path.Join(root, dname), 0777); err != nil {
			log.Printf("Error Mkdir2 %v in namespace.CreateFSNamespace: %v", path.Join(root, dname), err)
			return err
		}
	}
	files := []string{
		"dev/urandom",
	}
	for _, fname := range files {
		if f, err := os.Create(path.Join(root, fname)); err != nil {
			log.Printf("Error Create %v in namespace.CreateFSNamespace: %v", path.Join(root, fname), err)
			return err
		} else {
			if err := f.Close(); err != nil {
				log.Printf("Error Close %v in namespace.CreateFSNamespace: %v", path.Join(root, fname), err)
				return err
			}
		}
	}
	return nil
}

func isolateFSNamespace(newRoot string) error {
	oldRootMnt := "old_root"

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

	// Mount /dev/urandom
	if err := syscall.Mount("/dev/urandom", path.Join(newRoot, "dev/urandom"), "none", syscall.MS_BIND, ""); err != nil {
		log.Printf("failed to mount /dev/urandom: %v", err)
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
	if err := os.RemoveAll(oldRootMnt); err != nil {
		log.Printf("failed to remove old root filesystem: %v", err)
		return err
	}

	return nil
}

func destroyFSNamespace(fsRoot string) error {
	if err := os.RemoveAll(fsRoot); err != nil {
		log.Printf("Error RemoveAll in namespace.Destroy: %v", err)
		return err
	}
	return nil
}

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

func setupFs(newRoot string) error {
	oldRootMnt := "old_root"

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
		log.Printf("failed to mount /dev/etc: %v", err)
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
	if err := os.RemoveAll(oldRootMnt); err != nil {
		log.Printf("failed to remove old root filesystem: %v", err)
		return err
	}

	return nil
}
