package session

import (
	"log"

	np "ulambda/ninep"
)

func (s *Session) Dispatch(msg np.Tmsg) (np.Tmsg, *np.Rerror) {
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
		// If the leader proposed this detach message.
		log.Printf("lid:%v propid:%v", req.LeadId, req.PropId)
		if req.LeadId == req.PropId {
			log.Printf("lid:%v propid:%v detach!", req.LeadId, req.PropId)
			s.protsrv.Detach()
		}
		return *reply, nil
	default:
		return nil, np.MkErr(np.TErrUnknownMsg, msg).Rerror()
	}
}
