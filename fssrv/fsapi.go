package fssrv

import (
	np "ulambda/ninep"
)

type VersionFs interface {
	Version(*FsConn, np.Tversion, *np.Rversion) error
}

type AuthFs interface {
	Auth(*FsConn, np.Tauth, *np.Rauth) error
}

type FlushFs interface {
	Flush(*FsConn, np.Tflush, *np.Rflush) error
}

type AttachFs interface {
	Attach(*FsConn, np.Tattach, *np.Rattach) error
}

type WalkFs interface {
	Walk(*FsConn, np.Twalk, *np.Rwalk) error
}

type CreateFs interface {
	Create(*FsConn, np.Tcreate, *np.Rcreate) error
}

type OpenFs interface {
	Open(*FsConn, np.Topen, *np.Ropen) error
}

type ClunkFs interface {
	Clunk(*FsConn, np.Tclunk, *np.Rclunk) error
}

type ReadFs interface {
	Read(*FsConn, np.Tread, *np.Rread) error
}

type WriteFs interface {
	Write(*FsConn, np.Twrite, *np.Rwrite) error
}
