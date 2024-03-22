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
	sp "sigmaos/sigmap"
)

const (
	// binfsd mounts itself here:
	BINFSMNT = "/mnt/binfs/"

	// The directory /tmp/sigmaos-bin/realms/<realm> in the host file
	// system is mounted here by uprocd:
	BINCACHE = sp.SIGMAHOME + "/bin/user/"

	DEBUG = false
)

func BinPath(program, buildtag string) string {
	return BINFSMNT + program + ":" + buildtag
}

func binCachePath(program string) string {
	return BINCACHE + program
}

func binPathParse(pn string) (string, string) {
	p := strings.Split(pn, ":")
	return p[0], p[1]
}

type binFsRoot struct {
	Path     string // the directory that holds cached binaries
	bincache *bincache
}

func (r *binFsRoot) newNode(parent *fs.Inode, name string, sz int64) fs.InodeEmbedder {
	n := &binFsNode{
		RootData: r,
		name:     name,
		sz:       sz,
	}
	return n
}

type binFsNode struct {
	fs.Inode

	RootData *binFsRoot
	name     string
	sz       int64
}

func (n *binFsNode) String() string {
	return fmt.Sprintf("{N %q}", n.path())
}

func newBinRoot(kernelId string, sc *sigmaclnt.SigmaClnt) (fs.InodeEmbedder, error) {
	var st syscall.Stat_t
	err := syscall.Stat(BINCACHE, &st)
	if err != nil {
		return nil, err
	}
	root := &binFsRoot{
		bincache: newBinCache(kernelId, sc),
	}

	return root.newNode(nil, "", 0), nil
}

func RunBinFS(kernelId, dir string) error {
	pe := proc.GetProcEnv()

	db.DPrintf(db.BINSRV, "MkDir %q", BINFSMNT)

	if err := os.MkdirAll(BINFSMNT, 0750); err != nil {
		return err
	}

	db.DPrintf(db.BINSRV, "%s", db.LsDir(BINCACHE))

	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		return err
	}

	loopbackRoot, err := newBinRoot(kernelId, sc)
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

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		db.DPrintf(db.BINSRV, "terminate\n")
		server.Unmount()
	}()

	server.Wait()
	db.DPrintf(db.ALWAYS, "Wait returned\n")
	return nil
}
