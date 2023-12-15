package sigmaos

import (
	path "sigmaos/path"
	sp "sigmaos/sigmap"
)

type Watch func(string, error)

type SigmaOS interface {
	// Core interface

	Close(int) error
	Stat(string) (*sp.Stat, error)
	Create(string, sp.Tperm, sp.Tmode) (int, error)
	Open(string, sp.Tmode) (int, error)
	Rename(string, string) error
	Remove(string) error
	GetFile(string) ([]byte, error)
	PutFile(string, sp.Tperm, sp.Tmode, []byte, sp.Toffset, sp.TleaseId) (sp.Tsize, error)
	Read(int, sp.Tsize) ([]byte, error)
	Write(int, []byte) (sp.Tsize, error)
	Seek(int, sp.Toffset) error

	// Ephemeral
	CreateEphemeral(string, sp.Tperm, sp.Tmode, sp.TleaseId, sp.Tfence) (int, error)
	ClntId() sp.TclntId

	// Fences
	FenceDir(string, sp.Tfence) error
	WriteFence(int, []byte, sp.Tfence) (sp.Tsize, error)

	// RPC
	WriteRead(int, []byte) ([]byte, error)

	// Watches
	OpenWatch(string, sp.Tmode, Watch) (int, error)
	SetDirWatch(int, string, Watch) error
	SetRemoveWatch(string, Watch) error

	// Mounting
	MountTree(sp.Taddrs, string, string) error
	IsLocalMount(sp.Tmount) bool
	SetLocalMount(*sp.Tmount, string)
	PathLastMount(string) (path.Path, path.Path, error)
	GetNamedMount() sp.Tmount
	NewRootMount(string, string) error
	Mounts() []string

	// Debugging
	DetachAll() error
	Detach(string) error
	Disconnect(string) error
}
