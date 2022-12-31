package container

import (
	"log"
	"os"
	"path"
	"syscall"

	db "sigmaos/debug"
	"sigmaos/rand"
)

func pivotFs(rootfs string) error {
	oldRootMnt := "old_root" + rand.String(8)

	db.DPrintf(db.CONTAINER, "setupFs %s\n", rootfs)

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

	// Mount /sys for /sys/devices/system/cpu/online; XXX exclude
	// /sys/firmware; others?
	if err := syscall.Mount("/sys", path.Join(rootfs, "sys"), "sysfs", 0, ""); err != nil {
		log.Printf("failed to mount /sys err %v", err)
		return err
	}

	// Mount /dev for /dev/urandom and /dev/null
	if err := syscall.Mount("/dev/urandom", path.Join(rootfs, "dev/urandom"), "none", syscall.MS_BIND, ""); err != nil {
		log.Printf("failed to mount /dev/urandom err %v", err)
		return err
	}
	if err := syscall.Mount("/dev/null", path.Join(rootfs, "dev/null"), "none", syscall.MS_BIND, ""); err != nil {
		log.Printf("failed to mount /dev/null err %v", err)
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
