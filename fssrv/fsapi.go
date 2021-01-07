package fssrv

import (
	np "ulambda/ninep"
)

type FsConn interface {
	Version(np.Tversion, *np.Rversion) error
	Auth(np.Tauth, *np.Rauth) error
	Flush(np.Tflush, *np.Rflush) error
	Attach(np.Tattach, *np.Rattach) error
	Walk(np.Twalk, *np.Rwalk) error
	Create(np.Tcreate, *np.Rcreate) error
	Open(np.Topen, *np.Ropen) error
	Clunk(np.Tclunk, *np.Rclunk) error
	Read(np.Tread, *np.Rread) error
	Write(np.Twrite, *np.Rwrite) error
}
