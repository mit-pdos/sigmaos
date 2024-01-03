package sessstatesrv

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

func (s *Session) Dispatch(msg sessp.Tmsg, data []byte) (sessp.Tmsg, []byte, sp.TclntId, *sp.Rerror) {
	if s.IsClosed() {
		db.DPrintf(db.SESS_STATE_SRV_ERR, "Sess %v is closed; reject %v\n", s.Sid, msg.Type())
		err := serr.NewErr(serr.TErrClosed, fmt.Sprintf("session %v", s.Sid))
		return nil, nil, sp.NoClntId, sp.NewRerrorSerr(err)
	}
	switch req := msg.(type) {
	case *sp.Tversion:
		reply := &sp.Rversion{}
		err := s.protsrv.Version(req, reply)
		return reply, nil, sp.NoClntId, err
	case *sp.Tauth:
		reply := &sp.Rauth{}
		err := s.protsrv.Auth(req, reply)
		return reply, nil, sp.NoClntId, err
	case *sp.Tattach:
		reply := &sp.Rattach{}
		cid, err := s.protsrv.Attach(req, reply, s.attachClnt)
		if cid != sp.NoClntId {
			s.AddClnt(cid)
		}
		return reply, nil, sp.NoClntId, err
	case *sp.Twalk:
		reply := &sp.Rwalk{}
		err := s.protsrv.Walk(req, reply)
		return reply, nil, sp.NoClntId, err
	case *sp.Topen:
		reply := &sp.Ropen{}
		err := s.protsrv.Open(req, reply)
		return reply, nil, sp.NoClntId, err
	case *sp.Twatch:
		reply := &sp.Ropen{}
		err := s.protsrv.Watch(req, reply)
		return reply, nil, sp.NoClntId, err
	case *sp.Tcreate:
		reply := &sp.Rcreate{}
		err := s.protsrv.Create(req, reply)
		return reply, nil, sp.NoClntId, err
	case *sp.TreadF:
		reply := &sp.Rread{}
		data, err := s.protsrv.ReadF(req, reply)
		return reply, data, sp.NoClntId, err
	case *sp.TwriteF:
		reply := &sp.Rwrite{}
		err := s.protsrv.WriteF(req, data, reply)
		return reply, nil, sp.NoClntId, err
	case *sp.Tclunk:
		reply := &sp.Rclunk{}
		err := s.protsrv.Clunk(req, reply)
		return reply, nil, sp.NoClntId, err
	case *sp.Tremove:
		reply := &sp.Rremove{}
		err := s.protsrv.Remove(req, reply)
		return reply, nil, sp.NoClntId, err
	case *sp.Tremovefile:
		reply := &sp.Rremove{}
		err := s.protsrv.RemoveFile(req, reply)
		return reply, nil, sp.NoClntId, err
	case *sp.Tstat:
		reply := &sp.Rstat{}
		err := s.protsrv.Stat(req, reply)
		return reply, nil, sp.NoClntId, err
	case *sp.Twstat:
		reply := &sp.Rwstat{}
		err := s.protsrv.Wstat(req, reply)
		return reply, nil, sp.NoClntId, err
	case *sp.Trenameat:
		reply := &sp.Rrenameat{}
		err := s.protsrv.Renameat(req, reply)
		return reply, nil, sp.NoClntId, err
	case *sp.Tgetfile:
		reply := &sp.Rread{}
		data, err := s.protsrv.GetFile(req, reply)
		return reply, data, sp.NoClntId, err
	case *sp.Tputfile:
		reply := &sp.Rwrite{}
		err := s.protsrv.PutFile(req, data, reply)
		return reply, nil, sp.NoClntId, err
	case *sp.Tdetach:
		reply := &sp.Rdetach{}
		err := s.protsrv.Detach(req, reply, s.detachClnt)
		return reply, nil, req.TclntId(), err
	case *sp.Theartbeat:
		reply := &sp.Rheartbeat{}
		reply.Sids = req.Sids
		return reply, nil, sp.NoClntId, nil
	case *sp.Twriteread:
		reply := &sp.Rread{}
		data, err := s.protsrv.WriteRead(req, data, reply)
		return reply, data, sp.NoClntId, err
	default:
		db.DPrintf(db.ALWAYS, "Unexpected type: %v", msg)
		return nil, nil, sp.NoClntId, sp.NewRerrorSerr(serr.NewErr(serr.TErrUnknownMsg, msg))
	}
}
