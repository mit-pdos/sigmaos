// Package sigmaos defines the file API of SigmaOS
package sigmaos

import (
	path "sigmaos/path"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type Twait bool

const (
	O_NOW  Twait = false
	O_WAIT Twait = true
)

type PathClntAPI interface {
	GetFile(pn string, principal *sp.Tprincipal, mode sp.Tmode, off sp.Toffset, cnt sp.Tsize, f *sp.Tfence) ([]byte, error)
	Stat(name string, principal *sp.Tprincipal) (*sp.Stat, error)
}

type FileAPI interface {
	// Core interface

	CloseFd(fd int) error
	Stat(path string) (*sp.Stat, error)
	Create(path string, p sp.Tperm, m sp.Tmode) (int, error)

	// If w, then wait until path exists before opening it
	Open(path string, m sp.Tmode, w Twait) (int, error)

	Rename(srcpath string, dstpath string) error
	Remove(path string) error
	GetFile(path string) ([]byte, error)
	PutFile(path string, p sp.Tperm, m sp.Tmode, d []byte, o sp.Toffset, l sp.TleaseId) (sp.Tsize, error)
	Read(fd int, b []byte) (sp.Tsize, error)
	Write(fd int, d []byte) (sp.Tsize, error)
	Pread(fd int, b []byte, o sp.Toffset) (sp.Tsize, error)
	Seek(fd int, o sp.Toffset) error

	// Ephemeral
	CreateEphemeral(path string, p sp.Tperm, m sp.Tmode, l sp.TleaseId, f sp.Tfence) (int, error)
	ClntId() sp.TclntId

	// Fences
	FenceDir(path string, f sp.Tfence) error
	WriteFence(fd int, d []byte, f sp.Tfence) (sp.Tsize, error)

	// RPC
	WriteRead(fd int, iniov sessp.IoVec, outiov sessp.IoVec) error

	// Wait unil directory changes
	DirWait(fd int) error

	// Mounting
	MountTree(ep *sp.Tendpoint, tree, mount string) error
	IsLocalMount(ep *sp.Tendpoint) (bool, error)
	PathLastMount(path string) (path.Path, path.Path, error)
	GetNamedEndpoint() (*sp.Tendpoint, error)
	GetNamedEndpointRealm(realm sp.Trealm) (*sp.Tendpoint, error)
	NewRootMount(path string, epname string) error

	// Done using SigmaOS, which detaches from any endpointed servers
	// (which removes ephemeral files) and may close the session with
	// those servers.
	Close() error

	// Debugging
	SetLocalMount(ep *sp.Tendpoint, port sp.Tport)
	Mounts() []string
	Detach(path string) error
	Disconnect(path string) error
	Disconnected() bool
}
