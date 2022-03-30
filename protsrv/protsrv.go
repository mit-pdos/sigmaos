package protsrv

import (
	np "ulambda/ninep"
)

type Isrvconn interface {
	Close()
}

type Conn struct {
	Conn    Isrvconn
	Replies chan *np.Fcall
}

type Fsrvfcall func(*np.Fcall, *Conn)

type FsServer interface {
	SrvFcall(*np.Fcall, *Conn)
	Snapshot() []byte
	Restore([]byte)
}

type Protsrv interface {
	Version(np.Tversion, *np.Rversion) *np.Rerror
	Auth(np.Tauth, *np.Rauth) *np.Rerror
	Flush(np.Tflush, *np.Rflush) *np.Rerror
	Attach(np.Tattach, *np.Rattach) *np.Rerror
	Walk(np.Twalk, *np.Rwalk) *np.Rerror
	Create(np.Tcreate, *np.Rcreate) *np.Rerror
	Open(np.Topen, *np.Ropen) *np.Rerror
	Watch(np.Twatch, *np.Ropen) *np.Rerror
	Clunk(np.Tclunk, *np.Rclunk) *np.Rerror
	Read(np.Tread, *np.Rread) *np.Rerror
	ReadV(np.TreadV, *np.Rread) *np.Rerror
	Write(np.Twrite, *np.Rwrite) *np.Rerror
	WriteV(np.TwriteV, *np.Rwrite) *np.Rerror
	Remove(np.Tremove, *np.Rremove) *np.Rerror
	RemoveFile(np.Tremovefile, *np.Rremove) *np.Rerror
	Stat(np.Tstat, *np.Rstat) *np.Rerror
	Wstat(np.Twstat, *np.Rwstat) *np.Rerror
	Renameat(np.Trenameat, *np.Rrenameat) *np.Rerror
	GetFile(np.Tgetfile, *np.Rgetfile) *np.Rerror
	SetFile(np.Tsetfile, *np.Rwrite) *np.Rerror
	PutFile(np.Tputfile, *np.Rwrite) *np.Rerror
	Detach()
	Snapshot() []byte
}

type MkProtServer func(FsServer, np.Tsession) Protsrv
type RestoreProtServer func(FsServer, []byte) Protsrv
