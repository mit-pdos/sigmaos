package protsrv

import (
	np "ulambda/ninep"
	"ulambda/session"
)

type FsServer interface {
	Connect() Protsrv
	SessionTable() *session.SessionTable
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
	Closed() bool
}

type MakeProtServer interface {
	MakeProtServer(FsServer) Protsrv
}

func Dispatch(p Protsrv, sess np.Tsession, msg np.Tmsg) (np.Tmsg, *np.Rerror) {
	switch req := msg.(type) {
	case np.Tversion:
		reply := &np.Rversion{}
		err := p.Version(sess, req, reply)
		return *reply, err
	case np.Tauth:
		reply := &np.Rauth{}
		err := p.Auth(sess, req, reply)
		return *reply, err
	case np.Tattach:
		reply := &np.Rattach{}
		err := p.Attach(sess, req, reply)
		return *reply, err
	case np.Tflush:
		reply := &np.Rflush{}
		err := p.Flush(sess, req, reply)
		return *reply, err
	case np.Twalk:
		reply := &np.Rwalk{}
		err := p.Walk(sess, req, reply)
		return *reply, err
	case np.Topen:
		reply := &np.Ropen{}
		err := p.Open(sess, req, reply)
		return *reply, err
	case np.Twatchv:
		reply := &np.Ropen{}
		err := p.WatchV(sess, req, reply)
		return *reply, err
	case np.Tcreate:
		reply := &np.Rcreate{}
		err := p.Create(sess, req, reply)
		return *reply, err
	case np.Tread:
		reply := &np.Rread{}
		err := p.Read(sess, req, reply)
		return *reply, err
	case np.Twrite:
		reply := &np.Rwrite{}
		err := p.Write(sess, req, reply)
		return *reply, err
	case np.Tclunk:
		reply := &np.Rclunk{}
		err := p.Clunk(sess, req, reply)
		return *reply, err
	case np.Tremove:
		reply := &np.Rremove{}
		err := p.Remove(sess, req, reply)
		return *reply, err
	case np.Tremovefile:
		reply := &np.Rremove{}
		err := p.RemoveFile(sess, req, reply)
		return *reply, err
	case np.Tstat:
		reply := &np.Rstat{}
		err := p.Stat(sess, req, reply)
		return *reply, err
	case np.Twstat:
		reply := &np.Rwstat{}
		err := p.Wstat(sess, req, reply)
		return *reply, err
	case np.Trenameat:
		reply := &np.Rrenameat{}
		err := p.Renameat(sess, req, reply)
		return *reply, err
	case np.Tgetfile:
		reply := &np.Rgetfile{}
		err := p.GetFile(sess, req, reply)
		return *reply, err
	case np.Tsetfile:
		reply := &np.Rwrite{}
		err := p.SetFile(sess, req, reply)
		return *reply, err
	case np.Tregister:
		reply := &np.Ropen{}
		err := p.Register(sess, req, reply)
		return *reply, err
	case np.Tderegister:
		reply := &np.Ropen{}
		err := p.Deregister(sess, req, reply)
		return *reply, err
	default:
		return np.ErrUnknownMsg, nil
	}
}
