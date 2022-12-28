package container

import (
	"log"
	"os"
	"path"
	"syscall"

	db "sigmaos/debug"
	// "sigmaos/proc"
)

var dirs = []string{
	"proc",
	"dev",
}

func setupFs(rootfs string) error {
	oldRootMnt := "old_root"

	db.DPrintf(db.CONTAINER, "setupFs %s\n", rootfs)

	for _, dname := range dirs {
		if err := os.Mkdir(path.Join(rootfs, dname), 0777); err != nil {
			log.Printf("MkDir %s err %v", path.Join(rootfs, dname), err)
			return err
		}
	}

	// Mount new file system as a mount point so we can pivot_root to it later
	if err := syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND, ""); err != nil {
		log.Printf("failed to mount new root filesystem: %v", err)
		return err
	}

	// Mount old file system
	if err := syscall.Mkdir(path.Join(rootfs, oldRootMnt), 0700); err != nil {
		log.Printf("failed to mkdir: %v", err)
		return err
	}

	// pivot_root
	if err := syscall.PivotRoot(rootfs, path.Join(rootfs, oldRootMnt)); err != nil {
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
