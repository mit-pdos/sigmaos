package sigmaprotsrv

import (
	"sigmaos/queue"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type Isrvconn interface {
}

type Conn interface {
	IsClosed() bool
	Close()
	CloseConnTest()
	GetReplyQueue() *queue.ReplyQueue
}

type Fsrvfcall func(*sessp.FcallMsg)

type SessServer interface {
	Register(sessp.Tclient, sessp.Tsession, Conn) *serr.Err
	Unregister(sessp.Tclient, sessp.Tsession, Conn)
	SrvFcall(*sessp.FcallMsg)
	Snapshot() []byte
	Restore([]byte)
}

type Protsrv interface {
	Version(*sp.Tversion, *sp.Rversion) *sp.Rerror
	Auth(*sp.Tauth, *sp.Rauth) *sp.Rerror
	Attach(*sp.Tattach, *sp.Rattach) *sp.Rerror
	Walk(*sp.Twalk, *sp.Rwalk) *sp.Rerror
	Create(*sp.Tcreate, *sp.Rcreate) *sp.Rerror
	Open(*sp.Topen, *sp.Ropen) *sp.Rerror
	Watch(*sp.Twatch, *sp.Ropen) *sp.Rerror
	Clunk(*sp.Tclunk, *sp.Rclunk) *sp.Rerror
	ReadV(*sp.TreadV, *sp.Rread) ([]byte, *sp.Rerror)
	WriteV(*sp.TwriteV, []byte, *sp.Rwrite) *sp.Rerror
	Remove(*sp.Tremove, *sp.Rremove) *sp.Rerror
	RemoveFile(*sp.Tremovefile, *sp.Rremove) *sp.Rerror
	Stat(*sp.Tstat, *sp.Rstat) *sp.Rerror
	Wstat(*sp.Twstat, *sp.Rwstat) *sp.Rerror
	Renameat(*sp.Trenameat, *sp.Rrenameat) *sp.Rerror
	GetFile(*sp.Tgetfile, *sp.Rread) ([]byte, *sp.Rerror)
	PutFile(*sp.Tputfile, []byte, *sp.Rwrite) *sp.Rerror
	WriteRead(*sp.Twriteread, []byte, *sp.Rread) ([]byte, *sp.Rerror)
	Detach(*sp.Rdetach, DetachF) *sp.Rerror
	Snapshot() []byte
}

type MkProtServer func(SessServer, sessp.Tsession) Protsrv
type RestoreProtServer func(SessServer, []byte) Protsrv

type DetachF func(sessp.Tsession)
