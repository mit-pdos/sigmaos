package session

import (
	np "ulambda/ninep"
)

func (s *Session) Dispatch(msg np.Tmsg) (np.Tmsg, *np.Rerror) {
	// Register a heartbeat. This should be safe to do even if this is a detach,
	// since, if it is a detach, the decision to kill the session has already
	// been made & confirmed (if the leader made the decision), and if the leader
	// didn't make the decision, the heartbeat which this detach produces won't
	// be replicated (adding to the leader's timer) anyway. In the worst case, if
	// we change leadership while a non-leader is trying to detach, the eventual
	// detach will just be delayed by a bit.
	s.sm.Heartbeats([]np.Tsession{s.Sid})
	switch req := msg.(type) {
	case np.Tversion:
		reply := &np.Rversion{}
		err := s.protsrv.Version(req, reply)
		return *reply, err
	case np.Tauth:
		reply := &np.Rauth{}
		err := s.protsrv.Auth(req, reply)
		return *reply, err
	case np.Tattach:
		reply := &np.Rattach{}
		err := s.protsrv.Attach(req, reply)
		return *reply, err
	case np.Tflush:
		reply := &np.Rflush{}
		err := s.protsrv.Flush(req, reply)
		return *reply, err
	case np.Twalk:
		reply := &np.Rwalk{}
		err := s.protsrv.Walk(req, reply)
		return *reply, err
	case np.Topen:
		reply := &np.Ropen{}
		err := s.protsrv.Open(req, reply)
		return *reply, err
	case np.Twatch:
		reply := &np.Ropen{}
		err := s.protsrv.Watch(req, reply)
		return *reply, err
	case np.Tcreate:
		reply := &np.Rcreate{}
		err := s.protsrv.Create(req, reply)
		return *reply, err
	case np.Tread:
		reply := &np.Rread{}
		err := s.protsrv.Read(req, reply)
		return *reply, err
	case np.Twrite:
		reply := &np.Rwrite{}
		err := s.protsrv.Write(req, reply)
		return *reply, err
	case np.Tclunk:
		reply := &np.Rclunk{}
		err := s.protsrv.Clunk(req, reply)
		return *reply, err
	case np.Tremove:
		reply := &np.Rremove{}
		err := s.protsrv.Remove(req, reply)
		return *reply, err
	case np.Tremovefile:
		reply := &np.Rremove{}
		err := s.protsrv.RemoveFile(req, reply)
		return *reply, err
	case np.Tstat:
		reply := &np.Rstat{}
		err := s.protsrv.Stat(req, reply)
		return *reply, err
	case np.Twstat:
		reply := &np.Rwstat{}
		err := s.protsrv.Wstat(req, reply)
		return *reply, err
	case np.Trenameat:
		reply := &np.Rrenameat{}
		err := s.protsrv.Renameat(req, reply)
		return *reply, err
	case np.Tgetfile:
		reply := &np.Rgetfile{}
		err := s.protsrv.GetFile(req, reply)
		return *reply, err
	case np.Tsetfile:
		reply := &np.Rwrite{}
		err := s.protsrv.SetFile(req, reply)
		return *reply, err
	case np.Tputfile:
		reply := &np.Rwrite{}
		err := s.protsrv.PutFile(req, reply)
		return *reply, err
	case np.Tmkfence:
		reply := &np.Rmkfence{}
		err := s.protsrv.MkFence(req, reply)
		return *reply, err
	case np.Trmfence:
		reply := &np.Ropen{}
		err := s.protsrv.RmFence(req, reply)
		return *reply, err
	case np.Tregfence:
		reply := &np.Ropen{}
		err := s.protsrv.RegFence(req, reply)
		return *reply, err
	case np.Tunfence:
		reply := &np.Ropen{}
		err := s.protsrv.UnFence(req, reply)
		return *reply, err
	case np.Tdetach:
		reply := &np.Rdetach{}
		// If the leader proposed this detach message, accept it.
		if req.LeadId == req.PropId {
			s.protsrv.Detach()
			s.sm.DetachSession(s.Sid)
		}
		return *reply, nil
	case np.Theartbeat:
		reply := &np.Rheartbeat{}
		return *reply, nil
	default:
		return nil, np.MkErr(np.TErrUnknownMsg, msg).Rerror()
	}
}
