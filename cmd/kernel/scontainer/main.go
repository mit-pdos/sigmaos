package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	db "sigmaos/debug"

	"golang.org/x/sys/unix"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v jail_path", os.Args[0])
	}

	path := os.Args[1]
	setupJail(path)

	fmt.Printf("ok\n")
	os.Stdout.Sync()

	// Keep the process, and by extension the mount namespace alive indefinitely.
	select {}
}

func setupJail(jailPath string) {
	err := os.MkdirAll(jailPath, 0777)
	if err != nil {
		panic(fmt.Errorf("error creating jail directory: %w", err))
	}

	dirs := []string{
		"oldroot",
		"lib",
		"lib64",
		"usr",
		"etc",
		"proc",
		"bin",
		"mnt",
		"dev/shm",
		"tmp/sigmaos-perf",
		"tmp/spproxyd",
		"tmp/python/pyproc",
		"tmp/python/package-cache",
		"tmp/python/cpython3.11",
	}

	files := []string{
		"dev/random",
		"dev/urandom",
		"tmp/sigma_fork.sock",
	}

	for _, dir := range dirs {
		path := filepath.Join(jailPath, dir)
		err = os.MkdirAll(path, 0777)
		if err != nil {
			panic(fmt.Errorf("error creating directory %v: %w", path, err))
		}
	}

	for _, file := range files {
		path := filepath.Join(jailPath, file)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			panic(fmt.Errorf("error creating file %v: %w", path, err))
		}
		f.Close()
	}

	// Mount namespaces are per-thread
	runtime.LockOSThread()

	// Unshare mount namespace
	err = unix.Unshare(unix.CLONE_NEWNS)
	if err != nil {
		panic(fmt.Errorf("error unsharing mount namespace: %w", err))
	}

	// Make mounts private (avoid propagation back to host)
	err = unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, "")
	if err != nil {
		panic(fmt.Errorf("error making mounts private: %w", err))
	}

	// Bind mount the jail directory to itself so that we can pivot to it later
	err = unix.Mount(jailPath, jailPath, "", unix.MS_BIND|unix.MS_REC, "")
	if err != nil {
		panic(fmt.Errorf("error binding jail path: %w", err))
	}

	// Bind mount necessary directories and files
	err = unix.Chdir(jailPath)
	if err != nil {
		panic(fmt.Errorf("error changing directory: %w", err))
	}

	mustMount := func(source string, target string, fstype string, flags uintptr, data string) {
		err := unix.Mount(source, target, fstype, flags, data)
		if err != nil {
			panic(fmt.Sprintf("error mounting %v to %v: %v", source, target, err))
		}
	}

	mustMount("/lib", "lib", "", unix.MS_BIND|unix.MS_RDONLY, "")
	mustMount("/lib64", "lib64", "", unix.MS_BIND|unix.MS_RDONLY, "")
	mustMount("/usr", "usr", "", unix.MS_BIND|unix.MS_RDONLY, "")
	mustMount("/etc", "etc", "", unix.MS_BIND|unix.MS_RDONLY, "")
	mustMount("/proc", "proc", "proc", 0, "")
	mustMount("/bin", "bin", "", unix.MS_BIND|unix.MS_RDONLY, "")

	mustMount("/mnt", "mnt", "", unix.MS_BIND, "")
	mustMount("/mnt/binfs", "mnt/binfs", "", unix.MS_BIND, "")

	mustMount("/dev/shm", "dev/shm", "", unix.MS_BIND, "")
	mustMount("/dev/random", "dev/random", "", unix.MS_BIND, "")
	mustMount("/dev/urandom", "dev/urandom", "", unix.MS_BIND, "")

	// TODO: Must protect using apparmor
	mustMount("/tmp/sigmaos-perf", "tmp/sigmaos-perf", "", unix.MS_BIND, "")
	mustMount("/tmp/spproxyd", "tmp/spproxyd", "", unix.MS_BIND|unix.MS_RDONLY, "")
	mustMount("/tmp/sigma_fork.sock", "tmp/sigma_fork.sock", "", unix.MS_BIND|unix.MS_RDONLY, "")

	// TODO: Use binfs instead of directly mounting the pyproc dir.
	//       Unfortunately, binfs currently doesn't support directories.
	mustMount("/home/sigmaos/bin/kernel/pyproc", "tmp/python/pyproc", "", unix.MS_BIND|unix.MS_RDONLY, "")
	mustMount("/tmp/python/package-cache", "tmp/python/package-cache", "", unix.MS_BIND|unix.MS_RDONLY, "")
	mustMount("/home/sigmaos/bin/kernel/cpython3.11", "tmp/python/cpython3.11", "", unix.MS_BIND|unix.MS_RDONLY, "")

	// Pivot root to the jail directory
	err = unix.PivotRoot(".", "oldroot")
	if err != nil {
		panic(fmt.Errorf("error pivoting root: %w", err))
	}

	err = os.Chdir("/")
	if err != nil {
		panic(fmt.Errorf("error changing directory to new root: %w", err))
	}

	// Unmount old root
	err = unix.Unmount("oldroot", unix.MNT_DETACH)
	if err != nil {
		panic(fmt.Errorf("error unmounting old root: %w", err))
	}

	err = os.RemoveAll("oldroot")
	if err != nil {
		panic(fmt.Errorf("error removing old root: %w", err))
	}

	// Remount new root as read only
	err = unix.Mount("/", "/", "", unix.MS_REMOUNT|unix.MS_RDONLY|unix.MS_BIND, "")
	if err != nil {
		panic(fmt.Errorf("error remounting new root as read-only: %w", err))
	}
}
