package scontainer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	db "sigmaos/debug"
	"sigmaos/sched/msched/proc/srv/binsrv"
	sp "sigmaos/sigmap"
)

// JailPath is the single jail directory every uproc on this node pivot_roots
// into. It used to be created per proc by the trampoline, but nearly all of it
// is read-only bind mounts of the container's own /lib, /usr, /etc, ... which
// are identical for every proc, and building it cost each proc ~16 mkdirs and
// ~10 mounts. What genuinely cannot be shared stays in the trampoline: /proc
// (each proc has its own PID namespace) and the conditional benchmarking and
// Python mounts.
//
// Sharing the jail is safe because each proc still runs in its own mount
// namespace (CLONE_NEWNS in StartSigmaContainer), so its pivot_root and its
// own mounts are private to it. It does mean procs no longer get a private
// root *directory*: a proc that writes a relative pathname after pivot_root
// writes into the shared jail, where the other procs can see it (and where it
// is no longer cleaned up on proc exit). Procs write their output through
// UX/S3 or /tmp/sigmaos-perf, so nothing does this today.
//
// Must match JAIL in rs/uproc-trampoline/src/main.rs.
var JailPath = filepath.Join(sp.SIGMAHOME, "jail", "uproc")

// Directories which serve as the jail's mount points, plus the jail root
// itself. mnt/binfs is missing on purpose: it comes with the read-only bind of
// /mnt, and cannot be created underneath it.
var jailDirs = []string{
	"",
	"oldroot",
	"lib",
	"lib64",
	"usr",
	"etc",
	"proc",
	"bin",
	"bin/user",
	"mnt",
	"dev",
	"dev/shm",
	"tmp",
	"tmp/sigmaos-perf",
	"home/sigmaos/python",
	"run",
}

type jailMount struct {
	src    string
	dst    string // relative to the jail root
	fstype string
	flags  uintptr
}

// The mounts every proc needs, with the flags each proc used to set up for
// itself: keep the read-only/read-write split as is, since some of these must
// stay writable (e.g. /dev/shm, and /tmp/sigmaos-perf below, through which
// procs hand back benchmark results).
var jailMounts = []jailMount{
	// E.g., execve /lib/ld-musl-x86_64.so.1
	{"/lib", "lib", "none", syscall.MS_BIND | syscall.MS_RDONLY},
	// E.g., openat "/lib64/ld-musl-x86_64.so.1" (links to /lib/)
	{"/lib64", "lib64", "none", syscall.MS_BIND | syscall.MS_RDONLY},
	// E.g., /usr/lib for shared libraries (e.g., /usr/lib/libseccomp.so.2)
	{"/usr", "usr", "none", syscall.MS_BIND | syscall.MS_RDONLY},
	// E.g., Open "/etc/localtime"
	{"/etc", "etc", "none", syscall.MS_BIND | syscall.MS_RDONLY},
	{"/dev/shm", "dev/shm", "none", syscall.MS_BIND},
	// The binary passed to exec has the path /mnt/binfs/<binary>. binfs is a
	// FUSE mount, so it has to be bound separately (the bind of /mnt is not
	// recursive), and it has to already be mounted when we get here, which is
	// why the jail is built lazily on the first proc rather than at startup.
	{"/mnt/", "mnt", "none", syscall.MS_BIND | syscall.MS_RDONLY},
	{"/mnt/binfs/", "mnt/binfs", "none", syscall.MS_BIND | syscall.MS_RDONLY},
	{"/tmp/", "tmp", "none", syscall.MS_BIND | syscall.MS_RDONLY},
}

// Python procs bind /dev/urandom and /dev/null over these; create them here so
// that concurrent Python procs don't race to create them in the shared jail.
var jailFiles = []string{"dev/urandom", "dev/null"}

var jailMu sync.Mutex
var jailMade bool

// EnsureJail creates the shared jail, once. It is called on the first proc
// rather than at procd startup because binfs, which procd mounts
// asynchronously, must already be mounted for the bind of /mnt/binfs to see
// it. That is the same ordering the per-proc jail relied on. A failed attempt
// is retried by the next proc (makeJail rebuilds from scratch), since a jail
// which cannot be built stops this node from running any proc at all.
func EnsureJail() error {
	jailMu.Lock()
	defer jailMu.Unlock()
	if jailMade {
		return nil
	}
	if err := makeJail(); err != nil {
		db.DPrintf(db.ALWAYS, "Error makeJail %v: %v", JailPath, err)
		return err
	}
	jailMade = true
	return nil
}

// isMounted reports whether pn is a mount point in this mount namespace.
func isMounted(pn string) bool {
	b, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error read mountinfo: %v", err)
		return false
	}
	// mountinfo reports mount points without a trailing slash (e.g. binsrv's
	// BINFSMNT has one).
	pn = filepath.Clean(pn)
	for _, l := range strings.Split(string(b), "\n") {
		// mountinfo: id parent major:minor root mountpoint ...
		if f := strings.Fields(l); len(f) > 4 && f[4] == pn {
			return true
		}
	}
	return false
}

// waitBinFs waits for binsrv to mount binfs. The jail binds it once for every
// proc, so unlike the per-proc jail it cannot pick up a binfs which is mounted
// after the jail is built.
func waitBinFs() error {
	const timeout = 10 * time.Second
	s := time.Now()
	for !isMounted(binsrv.BINFSMNT) {
		if time.Since(s) > timeout {
			return fmt.Errorf("%v not mounted after %v", binsrv.BINFSMNT, timeout)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if d := time.Since(s); d > time.Millisecond {
		db.DPrintf(db.ALWAYS, "EnsureJail: waited %v for %v", d, binsrv.BINFSMNT)
	}
	return nil
}

func makeJail() error {
	s := time.Now()

	if err := waitBinFs(); err != nil {
		return err
	}

	// A previous procd in this mount namespace may have left the jail mounted;
	// detach it (lazily, so any of its procs still running are unaffected)
	// rather than stacking a second copy of every mount on top of it.
	if err := syscall.Unmount(JailPath, syscall.MNT_DETACH); err == nil {
		db.DPrintf(db.ALWAYS, "EnsureJail: detached stale jail %v", JailPath)
	}

	for _, d := range jailDirs {
		if err := os.MkdirAll(filepath.Join(JailPath, d), 0755); err != nil {
			return err
		}
	}
	for _, f := range jailFiles {
		pn := filepath.Join(JailPath, f)
		fd, err := os.OpenFile(pn, os.O_RDONLY|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		fd.Close()
	}

	// pivot_root only accepts a mount point as the new root, so mount the jail
	// on itself...
	if err := syscall.Mount(JailPath, JailPath, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("mount jail %q: %v", JailPath, err)
	}
	// ...and make it private, both because pivot_root refuses a shared new
	// root and so that the mounts procs make inside their own namespace (e.g.
	// /proc) don't propagate back out here.
	if err := syscall.Mount("", JailPath, "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("make jail %q private: %v", JailPath, err)
	}
	for _, m := range jailMounts {
		dst := filepath.Join(JailPath, m.dst)
		if err := syscall.Mount(m.src, dst, m.fstype, m.flags, ""); err != nil {
			return fmt.Errorf("mount %q -> %q: %v", m.src, dst, err)
		}
	}
	db.DPrintf(db.ALWAYS, "EnsureJail %v: %v", JailPath, time.Since(s))
	return nil
}
