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
