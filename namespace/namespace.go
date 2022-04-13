package namespace

import (
	"log"
	"os"
	"os/exec"
	"path"
	"runtime/debug"
	"syscall"
)

const (
	NAMESPACE_DIR = "/tmp/ulambda/isolation"
)

func SetupProc(cmd *exec.Cmd) {
	// Set up new namespaces
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWIPC |
			//			syscall.CLONE_NEWNET | // XXX Causes tests to fail : need to figure out how to add to network namespace
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
}

func Isolate(fsRoot string) error {
	if err := createFSNamespace(fsRoot); err != nil {
		log.Printf("Error CreateFSNamespace in namespace.Isolate: %v", err)
		return err
	}
	if err := isolateFSNamespace(fsRoot); err != nil {
		log.Printf("Error IsolateFSNamespace in namespace.Isolate: %v", err)
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
