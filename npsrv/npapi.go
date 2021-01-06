package npsrv

import (
	np "ulambda/ninep"
)

type NpAPI interface {
	Version(np.Tversion, *np.Rversion) *np.Rerror
	Auth(np.Tauth, *np.Rauth) *np.Rerror
	Flush(np.Tflush, *np.Rflush) *np.Rerror
	Attach(np.Tattach, *np.Rattach) *np.Rerror
	Walk(np.Twalk, *np.Rwalk) *np.Rerror
	Create(np.Tcreate, *np.Rcreate) *np.Rerror
	Symlink(np.Tsymlink, *np.Rsymlink) *np.Rerror
	Readlink(np.Treadlink, *np.Rreadlink) *np.Rerror
	Open(np.Topen, *np.Ropen) *np.Rerror
	Clunk(np.Tclunk, *np.Rclunk) *np.Rerror
	Read(np.Tread, *np.Rread) *np.Rerror
	Write(np.Twrite, *np.Rwrite) *np.Rerror
	Stat(np.Tstat, *np.Rstat) *np.Rerror
}
