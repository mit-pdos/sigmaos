package sigmap

import (
	"sigmaos/sessp"
)

type Isrvconn interface {
}

type Conn interface {
	IsClosed() bool
	Close()
	CloseConnTest()
	GetReplyC() chan *sessp.FcallMsg
}

type Fsrvfcall func(*sessp.FcallMsg)

type SessServer interface {
	Register(sessp.Tclient, sessp.Tsession, Conn) *sessp.Err
	Unregister(sessp.Tclient, sessp.Tsession, Conn)
	SrvFcall(*sessp.FcallMsg)
	Snapshot() []byte
	Restore([]byte)
}

type Protsrv interface {
	Version(*Tversion, *Rversion) *Rerror
	Auth(*Tauth, *Rauth) *Rerror
	Attach(*Tattach, *Rattach) *Rerror
	Walk(*Twalk, *Rwalk) *Rerror
	Create(*Tcreate, *Rcreate) *Rerror
	Open(*Topen, *Ropen) *Rerror
	Watch(*Twatch, *Ropen) *Rerror
	Clunk(*Tclunk, *Rclunk) *Rerror
	ReadV(*TreadV, *Rread) ([]byte, *Rerror)
	WriteV(*TwriteV, []byte, *Rwrite) *Rerror
	Remove(*Tremove, *Rremove) *Rerror
	RemoveFile(*Tremovefile, *Rremove) *Rerror
	Stat(*Tstat, *Rstat) *Rerror
	Wstat(*Twstat, *Rwstat) *Rerror
	Renameat(*Trenameat, *Rrenameat) *Rerror
	GetFile(*Tgetfile, *Rread) ([]byte, *Rerror)
	SetFile(*Tsetfile, []byte, *Rwrite) *Rerror
	PutFile(*Tputfile, []byte, *Rwrite) *Rerror
	WriteRead(*Twriteread, []byte, *Rread) ([]byte, *Rerror)
	Detach(*Rdetach, DetachF) *Rerror
	Snapshot() []byte
}

type MkProtServer func(SessServer, sessp.Tsession) Protsrv
type RestoreProtServer func(SessServer, []byte) Protsrv

type DetachF func(sessp.Tsession)
