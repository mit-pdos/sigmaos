// The binsrv package serves sigmaos binaries to the linux kernel.  It
// fetches the binary using the sigmaos pathname and caches them
// locally.  This allow support demand paging: the kernel can start
// running the binary before the complete binary has been downloaded.
//
// binsrv is based on go-fuse's loopback.
package binsrv

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

const (
	BINFSMNT = "/mnt/binfs/"
	BINCACHE = "bin/cache"
	DEBUG    = false
)

type binFsRoot struct {
	// The path to the root of the underlying file system.
	Path string

	// The device on which the Path resides. This must be set if
	// the underlying filesystem crosses file systems.
	Dev uint64

	KernelId string

	Sc *sigmaclnt.SigmaClnt

	// NewNode returns a new InodeEmbedder to be used to respond
	// to a LOOKUP/CREATE/MKDIR/MKNOD opcode. If not set, use a
	// LoopbackNode.
	NewNode func(rootData *binFsRoot, parent *fs.Inode, name string, st *syscall.Stat_t) fs.InodeEmbedder
}

func (r *binFsRoot) newNode(parent *fs.Inode, name string, st *syscall.Stat_t) fs.InodeEmbedder {
	if r.NewNode != nil {
		return r.NewNode(r, parent, name, st)
	}
	n := &binFsNode{
		RootData: r,
	}
	n.waiters = sync.NewCond(&n.mu)
	return n
}

func (r *binFsRoot) idFromStat(st *syscall.Stat_t) fs.StableAttr {
	// We compose an inode number by the underlying inode, and
	// mixing in the device number. In traditional filesystems,
	// the inode numbers are small. The device numbers are also
	// small (typically 16 bit). Finally, we mask out the root
	// device number of the root, so a loopback FS that does not
	// encompass multiple mounts will reflect the inode numbers of
	// the underlying filesystem
	swapped := (uint64(st.Dev) << 32) | (uint64(st.Dev) >> 32)
	swappedRootDev := (r.Dev << 32) | (r.Dev >> 32)
	return fs.StableAttr{
		Mode: uint32(st.Mode),
		Gen:  1,
		// This should work well for traditional backing FSes,
		// not so much for other go-fuse FS-es
		Ino: (swapped ^ swappedRootDev) ^ st.Ino,
	}
}

type binFsNode struct {
	fs.Inode

	RootData *binFsRoot

	mu      sync.Mutex
	waiters *sync.Cond
	nwaiter int

	dl *downloader
}

func newBinRoot(rootPath, kernelId string, sc *sigmaclnt.SigmaClnt) (fs.InodeEmbedder, error) {
	var st syscall.Stat_t
	err := syscall.Stat(rootPath, &st)
	if err != nil {
		return nil, err
	}

	root := &binFsRoot{
		Path:     rootPath,
		Dev:      uint64(st.Dev),
		KernelId: kernelId,
		Sc:       sc,
	}

	return root.newNode(nil, "", &st), nil
}

func RunBinFS(kernelId string) error {
	pe := proc.GetProcEnv()

	if err := os.MkdirAll(BINFSMNT, 0750); err != nil {
		return err
	}

	if err := os.MkdirAll(BINCACHE, 0750); err != nil {
		return err
	}

	f, err := os.Create(BINCACHE + "/xxx")
	if err != nil {
		return err
	}
	f.Close()

	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		return err
	}

	loopbackRoot, err := newBinRoot(BINCACHE, kernelId, sc)
	if err != nil {
		return err
	}
	sec := time.Second
	opts := &fs.Options{
		AttrTimeout:  &sec,
		EntryTimeout: &sec,

		NullPermissions: true, // Leave file permissions on "000" files as-is

		MountOptions: fuse.MountOptions{
			Debug:  DEBUG,
			FsName: BINCACHE, // First column in "df -T": original dir
			Name:   "binfs",  // Second column in "df -T" will be shown as "fuse." + Name
		},
	}
	opts.MountOptions.Options = append(opts.MountOptions.Options, "ro")

	server, err := fs.Mount("/mnt/binfs", loopbackRoot, opts)
	if err != nil {
		return err
	}

	// ch := make(chan bool)
	// go func() {
	// 	if err := sc.WaitEvict(pe.GetPID()); err != nil {
	// 		db.DPrintf(db.ALWAYS, "WaitEvict err %v", err)
	// 	}
	// 	server.Unmount()
	// 	sc.ClntExitOK()
	// 	ch <- true
	// }()

	// if err := sc.Started(); err != nil {
	// 	db.DPrintf(db.ALWAYS, "Error Started: %v", err)
	// 	return err
	// }

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		server.Unmount()
	}()

	server.Wait()
	//<-ch
	//db.DPrintf(db.ALWAYS, "Wait returned\n")
	return nil
}
