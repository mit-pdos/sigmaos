// Package sigmaos defines the core API of SigmaOS
package sigmaos

import (
	path "sigmaos/path"
	sp "sigmaos/sigmap"
)

type Watch func(string, error)

type SigmaOS interface {
	// Core interface

	Close(fd int) error
	Stat(path string) (*sp.Stat, error)
	Create(path string, p sp.Tperm, m sp.Tmode) (int, error)
	Open(path string, m sp.Tmode) (int, error)
	Rename(srcpath string, dstpath string) error
	Remove(path string) error
	GetFile(path string) ([]byte, error)
	PutFile(path string, p sp.Tperm, m sp.Tmode, d []byte, o sp.Toffset, l sp.TleaseId) (sp.Tsize, error)
	Read(fd int, sz sp.Tsize) ([]byte, error)
	Write(fd int, d []byte) (sp.Tsize, error)
	Seek(fd int, o sp.Toffset) error

	// Ephemeral
	CreateEphemeral(path string, p sp.Tperm, m sp.Tmode, l sp.TleaseId, f sp.Tfence) (int, error)
	ClntId() sp.TclntId

	// Fences
	FenceDir(path string, f sp.Tfence) error
	WriteFence(fd int, d []byte, f sp.Tfence) (sp.Tsize, error)

	// RPC
	WriteRead(fd int, d []byte) ([]byte, error)

	// Watches

	// If OpenWatch returns Notfound for path, sigmaos will fire w
	// when path comes into existence.
	OpenWatch(path string, m sp.Tmode, w Watch) (int, error)
	// Blocks unil directory changes
	SetDirWatch(fd int, dir string, w Watch) error
	// If file exists, block until it doesn't exist
	SetRemoveWatch(path string, w Watch) error

	// Mounting
	MountTree(addrs sp.Taddrs, tree, mount string) error
	IsLocalMount(mnt sp.Tmount) bool
	SetLocalMount(mnt *sp.Tmount, port string)
	PathLastMount(path string) (path.Path, path.Path, error)
	GetNamedMount() sp.Tmount
	NewRootMount(path string, mntname string) error
	Mounts() []string

	// Debugging
	DetachAll() error
	Detach(path string) error
	Disconnect(path string) error
}
