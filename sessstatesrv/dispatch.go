package sessstatesrv

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/sessp"
    "sigmaos/serr"
	sp "sigmaos/sigmap"
)

func (s *Session) Dispatch(msg sessp.Tmsg, data []byte) (sessp.Tmsg, []byte, bool, *sp.Rerror) {
	// If another replica detached a session, and the client sent their request
	// to this replica (which proposed it through raft), raft may spit out some
	// ops after the detach is processed. Catch these by returning an error.
	if s.IsClosed() {
		db.DPrintf(db.SESS_STATE_SRV_ERR, "Sess %v is closed; reject %v\n", s.Sid, msg.Type())
		err := serr.MkErr(serr.TErrClosed, fmt.Sprintf("session %v", s.Sid))
		return nil, nil, true, sp.MkRerror(err)
	}
	switch req := msg.(type) {
	case *sp.Tversion:
		reply := &sp.Rversion{}
		err := s.protsrv.Version(req, reply)
		return reply, nil, false, err
	case *sp.Tauth:
		reply := &sp.Rauth{}
		err := s.protsrv.Auth(req, reply)
		return reply, nil, false, err
	case *sp.Tattach:
		reply := &sp.Rattach{}
		err := s.protsrv.Attach(req, reply)
		return reply, nil, false, err
	case *sp.Twalk:
		reply := &sp.Rwalk{}
		err := s.protsrv.Walk(req, reply)
		return reply, nil, false, err
	case *sp.Topen:
		reply := &sp.Ropen{}
		err := s.protsrv.Open(req, reply)
		return reply, nil, false, err
	case *sp.Twatch:
		reply := &sp.Ropen{}
		err := s.protsrv.Watch(req, reply)
		return reply, nil, false, err
	case *sp.Tcreate:
		reply := &sp.Rcreate{}
		err := s.protsrv.Create(req, reply)
		return reply, nil, false, err
	case *sp.TreadV:
		reply := &sp.Rread{}
		data, err := s.protsrv.ReadV(req, reply)
		return reply, data, false, err
	case *sp.TwriteV:
		reply := &sp.Rwrite{}
		err := s.protsrv.WriteV(req, data, reply)
		return reply, nil, false, err
	case *sp.Tclunk:
		reply := &sp.Rclunk{}
		err := s.protsrv.Clunk(req, reply)
		return reply, nil, false, err
	case *sp.Tremove:
		reply := &sp.Rremove{}
		err := s.protsrv.Remove(req, reply)
		return reply, nil, false, err
	case *sp.Tremovefile:
		reply := &sp.Rremove{}
		err := s.protsrv.RemoveFile(req, reply)
		return reply, nil, false, err
	case *sp.Tstat:
		reply := &sp.Rstat{}
		err := s.protsrv.Stat(req, reply)
		return reply, nil, false, err
	case *sp.Twstat:
		reply := &sp.Rwstat{}
		err := s.protsrv.Wstat(req, reply)
		return reply, nil, false, err
	case *sp.Trenameat:
		reply := &sp.Rrenameat{}
		err := s.protsrv.Renameat(req, reply)
		return reply, nil, false, err
	case *sp.Tgetfile:
		reply := &sp.Rread{}
		data, err := s.protsrv.GetFile(req, reply)
		return reply, data, false, err
	case *sp.Tsetfile:
		reply := &sp.Rwrite{}
		err := s.protsrv.SetFile(req, data, reply)
		return reply, nil, false, err
	case *sp.Tputfile:
		reply := &sp.Rwrite{}
		err := s.protsrv.PutFile(req, data, reply)
		return reply, nil, false, err
	case *sp.Tdetach:
		reply := &sp.Rdetach{}
		db.DPrintf(db.SESS_STATE_SRV, "Try to detach l %v p %v", req.LeadId, req.PropId)
		// If the leader proposed this detach message, accept it.
		if req.LeadId == req.PropId {
			err := s.protsrv.Detach(reply, s.detach)
			return reply, nil, true, err
		}
		return reply, nil, false, nil
	case *sp.Theartbeat:
		reply := &sp.Rheartbeat{}
		reply.Sids = req.Sids
		return reply, nil, false, nil
	case *sp.Twriteread:
		reply := &sp.Rread{}
		data, err := s.protsrv.WriteRead(req, data, reply)
		return reply, data, false, err
	default:
		db.DPrintf(db.ALWAYS, "Unexpected type: %v", msg)
		return nil, nil, false, sp.MkRerror(serr.MkErr(serr.TErrUnknownMsg, msg))
	}
}
