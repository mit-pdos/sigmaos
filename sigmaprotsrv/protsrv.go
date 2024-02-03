package sigmaprotsrv

import (
	"sigmaos/serr"
	"sigmaos/sessconn"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type Isrvconn interface {
}

type Conn interface {
	IsClosed() bool
	Close()
	CloseConnTest()
	GetReplyChan() chan *sessconn.PartMarshaledMsg
}

type Fsrvfcall func(*sessp.FcallMsg) *sessp.FcallMsg

type SessServer interface {
	Register(sessp.Tsession, Conn) *serr.Err
	Unregister(sessp.Tsession, Conn)
	SrvFcall(*sessp.FcallMsg) *sessp.FcallMsg
}

type Protsrv interface {
	Version(*sp.Tversion, *sp.Rversion) *sp.Rerror
	Auth(*sp.Tauth, *sp.Rauth) *sp.Rerror
	Attach(*sp.Tattach, *sp.Rattach) (sp.TclntId, *sp.Rerror)
	Walk(*sp.Twalk, *sp.Rwalk) *sp.Rerror
	Create(*sp.Tcreate, *sp.Rcreate) *sp.Rerror
	Open(*sp.Topen, *sp.Ropen) *sp.Rerror
	Watch(*sp.Twatch, *sp.Ropen) *sp.Rerror
	Clunk(*sp.Tclunk, *sp.Rclunk) *sp.Rerror
	ReadF(*sp.TreadF, *sp.Rread) ([]byte, *sp.Rerror)
	WriteF(*sp.TwriteF, []byte, *sp.Rwrite) *sp.Rerror
	Remove(*sp.Tremove, *sp.Rremove) *sp.Rerror
	RemoveFile(*sp.Tremovefile, *sp.Rremove) *sp.Rerror
	Stat(*sp.Tstat, *sp.Rstat) *sp.Rerror
	Wstat(*sp.Twstat, *sp.Rwstat) *sp.Rerror
	Renameat(*sp.Trenameat, *sp.Rrenameat) *sp.Rerror
	GetFile(*sp.Tgetfile, *sp.Rread) ([]byte, *sp.Rerror)
	PutFile(*sp.Tputfile, []byte, *sp.Rwrite) *sp.Rerror
	WriteRead(*sp.Twriteread, []byte, *sp.Rread) ([]byte, *sp.Rerror)
	Detach(*sp.Tdetach, *sp.Rdetach) *sp.Rerror
}

type NewProtServer func(SessServer, sessp.Tsession) Protsrv

type DetachSessF func(sessp.Tsession)
