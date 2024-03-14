// The binsrv package serves sigmaos binaries to the linux kernel.  It
// fetches the binary using the sigmaos pathname and caches them
// locally.  This allow support demand paging: the kernel can start
// running the binary before the complete binary has been downloaded.
//
// binsrv is based on go-fuse's loopback.
package binsrv

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

const (
	BINFSMNT = "/mnt/binfs/"
	BINCACHE = "bin/cache/"
	DEBUG    = false
)

func BinPath(program, buildtag string) string {
	return BINFSMNT + program + ":" + buildtag
}

func binPathParse(pn string) (string, string) {
	p := strings.Split(pn, ":")
	return p[0], p[1]
}

type binFsRoot struct {
	// The path to the directory that holds cached binaries
	Path     string
	bincache *bincache
}

func (r *binFsRoot) newNode(parent *fs.Inode, name string, st *syscall.Stat_t) fs.InodeEmbedder {
	n := &binFsNode{
		RootData: r,
		name:     name,
	}
	return n
}

type binFsNode struct {
	fs.Inode

	RootData *binFsRoot
	name     string
}

func (n *binFsNode) String() string {
	return fmt.Sprintf("{N %q}", n.path())
}

func newBinRoot(rootPath, kernelId string, sc *sigmaclnt.SigmaClnt) (fs.InodeEmbedder, error) {
	var st syscall.Stat_t
	err := syscall.Stat(rootPath, &st)
	if err != nil {
		return nil, err
	}

	root := &binFsRoot{
		Path:     rootPath,
		bincache: newBinCache(kernelId, sc),
	}

	return root.newNode(nil, "", &st), nil
}

func BinFsCacheDir(instance string) string {
	return BINCACHE + instance
}

func Cleanup(dir string) error {
	d := BinFsCacheDir(dir)
	db.DPrintf(db.BINSRV, "Cleanup %s", d)
	return os.RemoveAll(d)
}

func RunBinFS(kernelId, dir string) error {
	pe := proc.GetProcEnv()

	if err := os.MkdirAll(BINFSMNT, 0750); err != nil {
		return err
	}

	if err := os.MkdirAll(BINCACHE, 0750); err != nil {
		return err
	}

	d := BinFsCacheDir(dir)
	if err := os.MkdirAll(d, 0750); err != nil {
		return err
	}

	db.DPrintf(db.BINSRV, "%s", db.LsDir(d))

	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		return err
	}

	loopbackRoot, err := newBinRoot(d, kernelId, sc)
	if err != nil {
		return err
	}
	sec := 100 * time.Second
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
		db.DPrintf(db.BINSRV, "terminate\n")
		server.Unmount()
	}()

	server.Wait()
	//<-ch
	db.DPrintf(db.ALWAYS, "Wait returned\n")
	return nil
}
