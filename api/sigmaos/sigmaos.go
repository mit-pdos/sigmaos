// Package sigmaos defines the file API of SigmaOS
package sigmaos

import (
	"io"

	"sigmaos/path"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
)

type Twait bool

const (
	O_NOW  Twait = false
	O_WAIT Twait = true
)

type Watch func(error)

type FileAPI interface {
	// Core interface

	CloseFd(fd int) error
	Stat(path string) (*sp.Tstat, error)
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
	PreadRdr(fd int, o sp.Toffset, sz sp.Tsize) (io.ReadCloser, error)
	Seek(fd int, o sp.Toffset) error

	// Leases
	CreateLeased(path string, p sp.Tperm, m sp.Tmode, l sp.TleaseId, f sp.Tfence) (int, error)
	ClntId() sp.TclntId

	// Fences
	FenceDir(path string, f sp.Tfence) error
	WriteFence(fd int, d []byte, f sp.Tfence) (sp.Tsize, error)

	// RPC
	WriteRead(fd int, iniov sessp.IoVec, outiov sessp.IoVec) error

	// Watch for directory changes
	DirWatch(fd int) (int, error)

	// Mounting
	MountTree(ep *sp.Tendpoint, tree, mount string) error
	IsLocalMount(ep *sp.Tendpoint) (bool, error)
	PathLastMount(path string) (path.Tpathname, path.Tpathname, error)
	GetNamedEndpoint() (*sp.Tendpoint, error)
	GetNamedEndpointRealm(realm sp.Trealm) (*sp.Tendpoint, error)
	InvalidateNamedEndpointCacheEntryRealm(realm sp.Trealm) error
	NewRootMount(path string, epname string) error
	MountPathClnt(mnt string, pc PathClntAPI) error

	// Done using SigmaOS, which detaches from any mounted servers and
	// may close the session with those servers.
	Close() error

	// Debugging
	SetLocalMount(ep *sp.Tendpoint, port sp.Tport)
	Mounts() []string
	Detach(path string) error
	Disconnect(path string) error
	Disconnected() bool
}

type PathClntAPI interface {
	Open(pn string, principal *sp.Tprincipal, mode sp.Tmode, w Watch) (sp.Tfid, error)
	Create(p string, principal *sp.Tprincipal, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f *sp.Tfence) (sp.Tfid, error)
	ReadF(fid sp.Tfid, off sp.Toffset, b []byte, f *sp.Tfence) (sp.Tsize, error)
	PreadRdr(fid sp.Tfid, off sp.Toffset, len sp.Tsize) (io.ReadCloser, error)
	WriteF(fid sp.Tfid, off sp.Toffset, data []byte, f *sp.Tfence) (sp.Tsize, error)
	Clunk(fid sp.Tfid) error
}
