// The binsrv package serves sigmaos binaries to the linux kernel.  It
// fetches the binary from [uprocsrv] and [chunksrv], and caches them
// locally.  This allow support demand paging: the kernel can start
// running the binary before the complete binary has been downloaded.
//
// binsrv is based on go-fuse's loopback.
package binsrv

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	chunksrv "sigmaos/chunk/srv"
	// db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/uprocclnt"
)

const (
	// binfsd mounts itself here:
	BINFSMNT = "/mnt/binfs/"

	DEBUG = false
)

func BinPath(program string) string {
	return BINFSMNT + program
}

func binCachePath(program string) string {
	return chunksrv.BINPROC + program
}

type binFsRoot struct {
	Path     string // the directory that holds cached binaries
	bincache *bincache
}

func (r *binFsRoot) newNode(parent *fs.Inode, name string, sz sp.Tsize) fs.InodeEmbedder {
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
	sz       sp.Tsize
}

func (n *binFsNode) String() string {
	return fmt.Sprintf("{N %q}", n.path())
}

func newBinRoot(upds uprocclnt.UprocSrv) (fs.InodeEmbedder, error) {
	var st syscall.Stat_t
	err := syscall.Stat(chunksrv.BINPROC, &st)
	if err != nil {
		return nil, err
	}
	root := &binFsRoot{
		bincache: newBinCache(upds),
	}
	return root.newNode(nil, "", 0), nil
}

func mountBinFs(upds uprocclnt.UprocSrv) (*fuse.Server, error) {
	loopbackRoot, err := newBinRoot(upds)
	if err != nil {
		return nil, err
	}
	sec := time.Second
	opts := &fs.Options{
		AttrTimeout:  &sec,
		EntryTimeout: &sec,

		NullPermissions: true, // Leave file permissions on "000" files as-is

		MountOptions: fuse.MountOptions{
			Debug:  DEBUG,
			FsName: chunksrv.BINPROC, // First column in "df -T": original dir
			Name:   "binfs",          // Second column in "df -T" will be shown as "fuse." + Name
		},
	}
	opts.MountOptions.Options = append(opts.MountOptions.Options, "ro")

	server, err := fs.Mount("/mnt/binfs", loopbackRoot, opts)
	if err != nil {
		return nil, err
	}
	return server, nil
}

func StartBinFs(upds uprocclnt.UprocSrv) error {
	if err := os.MkdirAll(BINFSMNT, 0750); err != nil {
		return err
	}
	server, err := mountBinFs(upds)
	if err != nil {
		return err
	}
	server.Wait()
	return nil
}
