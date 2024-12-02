package spprotsrv

import (
	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type ProtSrv interface {
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
	Stat(*sp.Trstat, *sp.Rrstat) *sp.Rerror
	Wstat(*sp.Twstat, *sp.Rwstat) *sp.Rerror
	Renameat(*sp.Trenameat, *sp.Rrenameat) *sp.Rerror
	GetFile(*sp.Tgetfile, *sp.Rread) ([]byte, *sp.Rerror)
	PutFile(*sp.Tputfile, []byte, *sp.Rwrite) *sp.Rerror
	WriteRead(*sp.Twriteread, sessp.IoVec, *sp.Rread) (sessp.IoVec, *sp.Rerror)
	Detach(*sp.Tdetach, *sp.Rdetach) *sp.Rerror
}

type DetachSessF func(sessp.Tsession)

type Tsessop int

const (
	TSESS_NONE Tsessop = iota
	TSESS_ADD
	TSESS_DEL
)

func Dispatch(protsrv ProtSrv, msg sessp.Tmsg, iov sessp.IoVec) (sessp.Tmsg, sessp.IoVec, *sp.Rerror, Tsessop, sp.TclntId) {
	switch req := msg.(type) {
	case *sp.Tversion:
		reply := &sp.Rversion{}
		err := protsrv.Version(req, reply)
		return reply, nil, err, TSESS_NONE, sp.NoClntId
	case *sp.Tauth:
		reply := &sp.Rauth{}
		err := protsrv.Auth(req, reply)
		return reply, nil, err, TSESS_NONE, sp.NoClntId
	case *sp.Tattach:
		reply := &sp.Rattach{}
		cid, err := protsrv.Attach(req, reply)
		if cid != sp.NoClntId {
			return reply, nil, err, TSESS_ADD, cid
		} else {
			return reply, nil, err, TSESS_NONE, sp.NoClntId
		}
	case *sp.Twalk:
		reply := &sp.Rwalk{}
		err := protsrv.Walk(req, reply)
		return reply, nil, err, TSESS_NONE, sp.NoClntId
	case *sp.Topen:
		reply := &sp.Ropen{}
		err := protsrv.Open(req, reply)
		return reply, nil, err, TSESS_NONE, sp.NoClntId
	case *sp.Twatch:
		reply := &sp.Ropen{}
		err := protsrv.Watch(req, reply)
		return reply, nil, err, TSESS_NONE, sp.NoClntId
	case *sp.Tcreate:
		reply := &sp.Rcreate{}
		err := protsrv.Create(req, reply)
		return reply, nil, err, TSESS_NONE, sp.NoClntId
	case *sp.TreadF:
		reply := &sp.Rread{}
		data, err := protsrv.ReadF(req, reply)
		return reply, sessp.IoVec{data}, err, TSESS_NONE, sp.NoClntId
	case *sp.TwriteF:
		reply := &sp.Rwrite{}
		err := protsrv.WriteF(req, iov[0], reply)
		return reply, nil, err, TSESS_NONE, sp.NoClntId
	case *sp.Tclunk:
		reply := &sp.Rclunk{}
		err := protsrv.Clunk(req, reply)
		return reply, nil, err, TSESS_NONE, sp.NoClntId
	case *sp.Tremove:
		reply := &sp.Rremove{}
		err := protsrv.Remove(req, reply)
		return reply, nil, err, TSESS_NONE, sp.NoClntId
	case *sp.Tremovefile:
		reply := &sp.Rremove{}
		err := protsrv.RemoveFile(req, reply)
		return reply, nil, err, TSESS_NONE, sp.NoClntId
	case *sp.Trstat:
		reply := &sp.Rrstat{}
		err := protsrv.Stat(req, reply)
		return reply, nil, err, TSESS_NONE, sp.NoClntId
	case *sp.Twstat:
		reply := &sp.Rwstat{}
		err := protsrv.Wstat(req, reply)
		return reply, nil, err, TSESS_NONE, sp.NoClntId
	case *sp.Trenameat:
		reply := &sp.Rrenameat{}
		err := protsrv.Renameat(req, reply)
		return reply, nil, err, TSESS_NONE, sp.NoClntId
	case *sp.Tgetfile:
		reply := &sp.Rread{}
		data, err := protsrv.GetFile(req, reply)
		return reply, sessp.IoVec{data}, err, TSESS_NONE, sp.NoClntId
	case *sp.Tputfile:
		reply := &sp.Rwrite{}
		err := protsrv.PutFile(req, iov[0], reply)
		return reply, nil, err, TSESS_NONE, sp.NoClntId
	case *sp.Tdetach:
		reply := &sp.Rdetach{}
		err := protsrv.Detach(req, reply)
		return reply, nil, err, TSESS_DEL, req.TclntId()
	case *sp.Theartbeat:
		reply := &sp.Rheartbeat{}
		reply.Sids = req.Sids
		return reply, nil, nil, TSESS_NONE, sp.NoClntId
	case *sp.Twriteread:
		reply := &sp.Rread{}
		iov, err := protsrv.WriteRead(req, iov, reply)
		return reply, iov, err, TSESS_NONE, sp.NoClntId
	default:
		db.DPrintf(db.ALWAYS, "Unexpected type: %v", msg)
		return nil, nil, sp.NewRerrorSerr(serr.NewErr(serr.TErrUnknownMsg, msg)), TSESS_NONE, sp.NoClntId
	}
}
