package protsrv

import (
	np "ulambda/ninep"
)

type FsServer interface {
	Dispatch(sess np.Tsession, msg np.Tmsg) (np.Tmsg, *np.Rerror)
	Detach(np.Tsession)
}

type Protsrv interface {
	Version(np.Tsession, np.Tversion, *np.Rversion) *np.Rerror
	Auth(np.Tsession, np.Tauth, *np.Rauth) *np.Rerror
	Flush(np.Tsession, np.Tflush, *np.Rflush) *np.Rerror
	Attach(np.Tsession, np.Tattach, *np.Rattach) *np.Rerror
	Walk(np.Tsession, np.Twalk, *np.Rwalk) *np.Rerror
	Create(np.Tsession, np.Tcreate, *np.Rcreate) *np.Rerror
	Open(np.Tsession, np.Topen, *np.Ropen) *np.Rerror
	WatchV(np.Tsession, np.Twatchv, *np.Ropen) *np.Rerror
	Clunk(np.Tsession, np.Tclunk, *np.Rclunk) *np.Rerror
	Read(np.Tsession, np.Tread, *np.Rread) *np.Rerror
	Write(np.Tsession, np.Twrite, *np.Rwrite) *np.Rerror
	Remove(np.Tsession, np.Tremove, *np.Rremove) *np.Rerror
	RemoveFile(np.Tsession, np.Tremovefile, *np.Rremove) *np.Rerror
	Stat(np.Tsession, np.Tstat, *np.Rstat) *np.Rerror
	Wstat(np.Tsession, np.Twstat, *np.Rwstat) *np.Rerror
	Renameat(np.Tsession, np.Trenameat, *np.Rrenameat) *np.Rerror
	GetFile(np.Tsession, np.Tgetfile, *np.Rgetfile) *np.Rerror
	SetFile(np.Tsession, np.Tsetfile, *np.Rwrite) *np.Rerror
	Register(np.Tsession, np.Tregister, *np.Ropen) *np.Rerror
	Deregister(np.Tsession, np.Tderegister, *np.Ropen) *np.Rerror
	Detach(np.Tsession)
}

type MkProtServer func(FsServer) Protsrv
