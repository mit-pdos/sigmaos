package fssrv

import (
	np "ulambda/ninep"
)

type VersionFs interface {
	Version(np.Tversion, *np.Rversion) error
}

type AuthFs interface {
	Auth(np.Tauth, *np.Rauth) error
}

type FlushFs interface {
	Flush(np.Tflush, *np.Rflush) error
}

type AttachFs interface {
	Attach(np.Tattach, *np.Rattach) error
}

type WalkFs interface {
	Walk(np.Twalk, *np.Rwalk) error
}

type CreateFs interface {
	Create(np.Tcreate, *np.Rcreate) error
}

type SymlinkFs interface {
	Symlink(np.Tsymlink, *np.Rsymlink) error
}

type ReadlinkFs interface {
	Readlink(np.Treadlink, *np.Rreadlink) error
}

type OpenFs interface {
	Open(np.Topen, *np.Ropen) error
}

type ClunkFs interface {
	Clunk(np.Tclunk, *np.Rclunk) error
}

type ReadFs interface {
	Read(np.Tread, *np.Rread) error
}

type WriteFs interface {
	Write(np.Twrite, *np.Rwrite) error
}
