package sessstatesrv

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/fcall"
	np "sigmaos/sigmap"
)

func (s *Session) Dispatch(msg fcall.Tmsg, data []byte) (fcall.Tmsg, []byte, bool, *np.Rerror) {
	// If another replica detached a session, and the client sent their request
	// to this replica (which proposed it through raft), raft may spit out some
	// ops after the detach is processed. Catch these by returning an error.
	if s.IsClosed() {
		db.DPrintf("SESSION_ERR", "Sess %v is closed; reject %v\n", s.Sid, msg.Type())
		err := fcall.MkErr(fcall.TErrClosed, fmt.Sprintf("session %v", s.Sid))
		return nil, nil, true, np.MkRerror(err)
	}
	switch req := msg.(type) {
	case *np.Tversion:
		reply := &np.Rversion{}
		err := s.protsrv.Version(req, reply)
		return reply, nil, false, err
	case *np.Tauth:
		reply := &np.Rauth{}
		err := s.protsrv.Auth(req, reply)
		return reply, nil, false, err
	case *np.Tattach:
		reply := &np.Rattach{}
		err := s.protsrv.Attach(req, reply)
		return reply, nil, false, err
	case *np.Twalk:
		reply := &np.Rwalk{}
		err := s.protsrv.Walk(req, reply)
		return reply, nil, false, err
	case *np.Topen:
		reply := &np.Ropen{}
		err := s.protsrv.Open(req, reply)
		return reply, nil, false, err
	case *np.Twatch:
		reply := &np.Ropen{}
		err := s.protsrv.Watch(req, reply)
		return reply, nil, false, err
	case *np.Tcreate:
		reply := &np.Rcreate{}
		err := s.protsrv.Create(req, reply)
		return reply, nil, false, err
	case *np.TreadV:
		reply := &np.Rread{}
		data, err := s.protsrv.ReadV(req, reply)
		return reply, data, false, err
	case *np.TwriteV:
		reply := &np.Rwrite{}
		err := s.protsrv.WriteV(req, data, reply)
		return reply, nil, false, err
	case *np.Tclunk:
		reply := &np.Rclunk{}
		err := s.protsrv.Clunk(req, reply)
		return reply, nil, false, err
	case *np.Tremove:
		reply := &np.Rremove{}
		err := s.protsrv.Remove(req, reply)
		return reply, nil, false, err
	case *np.Tremovefile:
		reply := &np.Rremove{}
		err := s.protsrv.RemoveFile(req, reply)
		return reply, nil, false, err
	case *np.Tstat:
		reply := &np.Rstat{}
		err := s.protsrv.Stat(req, reply)
		return reply, nil, false, err
	case *np.Twstat:
		reply := &np.Rwstat{}
		err := s.protsrv.Wstat(req, reply)
		return reply, nil, false, err
	case *np.Trenameat:
		reply := &np.Rrenameat{}
		err := s.protsrv.Renameat(req, reply)
		return reply, nil, false, err
	case *np.Tgetfile:
		reply := &np.Rread{}
		data, err := s.protsrv.GetFile(req, reply)
		return reply, data, false, err
	case *np.Tsetfile:
		reply := &np.Rwrite{}
		err := s.protsrv.SetFile(req, reply)
		return reply, nil, false, err
	case *np.Tputfile:
		reply := &np.Rwrite{}
		err := s.protsrv.PutFile(req, reply)
		return reply, nil, false, err
	case *np.Tdetach:
		reply := &np.Rdetach{}
		db.DPrintf("SESSION", "Try to detach l %v p %v", req.LeadId, req.PropId)
		// If the leader proposed this detach message, accept it.
		if req.LeadId == req.PropId {
			err := s.protsrv.Detach(reply, s.detach)
			return reply, nil, true, err
		}
		return reply, nil, false, nil
	case *np.Theartbeat:
		reply := &np.Rheartbeat{}
		reply.Sids = req.Sids
		return reply, nil, false, nil
	case *np.Twriteread:
		reply := &np.Rread{}
		data, err := s.protsrv.WriteRead(req, reply)
		return reply, data, false, err
	default:
		db.DPrintf(db.ALWAYS, "Unexpected type: %v", msg)
		return nil, nil, false, np.MkRerror(fcall.MkErr(fcall.TErrUnknownMsg, msg))
	}
}
