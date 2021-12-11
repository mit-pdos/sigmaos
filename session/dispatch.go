package session

import (
	// "log"

	np "ulambda/ninep"
)

func (s *Session) Dispatch(sess np.Tsession, msg np.Tmsg) (np.Tmsg, *np.Rerror) {
	// log.Printf("dipatch %v %v\n", sess, msg)
	switch req := msg.(type) {
	case np.Tversion:
		reply := &np.Rversion{}
		err := s.protsrv.Version(sess, req, reply)
		return *reply, err
	case np.Tauth:
		reply := &np.Rauth{}
		err := s.protsrv.Auth(sess, req, reply)
		return *reply, err
	case np.Tattach:
		reply := &np.Rattach{}
		err := s.protsrv.Attach(sess, req, reply)
		return *reply, err
	case np.Tflush:
		reply := &np.Rflush{}
		err := s.protsrv.Flush(sess, req, reply)
		return *reply, err
	case np.Twalk:
		reply := &np.Rwalk{}
		err := s.protsrv.Walk(sess, req, reply)
		return *reply, err
	case np.Topen:
		reply := &np.Ropen{}
		err := s.protsrv.Open(sess, req, reply)
		return *reply, err
	case np.Twatchv:
		reply := &np.Ropen{}
		err := s.protsrv.WatchV(sess, req, reply)
		return *reply, err
	case np.Tcreate:
		reply := &np.Rcreate{}
		err := s.protsrv.Create(sess, req, reply)
		return *reply, err
	case np.Tread:
		reply := &np.Rread{}
		err := s.protsrv.Read(sess, req, reply)
		return *reply, err
	case np.Twrite:
		reply := &np.Rwrite{}
		err := s.protsrv.Write(sess, req, reply)
		return *reply, err
	case np.Tclunk:
		reply := &np.Rclunk{}
		err := s.protsrv.Clunk(sess, req, reply)
		return *reply, err
	case np.Tremove:
		reply := &np.Rremove{}
		err := s.protsrv.Remove(sess, req, reply)
		return *reply, err
	case np.Tremovefile:
		reply := &np.Rremove{}
		err := s.protsrv.RemoveFile(sess, req, reply)
		return *reply, err
	case np.Tstat:
		reply := &np.Rstat{}
		err := s.protsrv.Stat(sess, req, reply)
		return *reply, err
	case np.Twstat:
		reply := &np.Rwstat{}
		err := s.protsrv.Wstat(sess, req, reply)
		return *reply, err
	case np.Trenameat:
		reply := &np.Rrenameat{}
		err := s.protsrv.Renameat(sess, req, reply)
		return *reply, err
	case np.Tgetfile:
		reply := &np.Rgetfile{}
		err := s.protsrv.GetFile(sess, req, reply)
		return *reply, err
	case np.Tsetfile:
		reply := &np.Rwrite{}
		err := s.protsrv.SetFile(sess, req, reply)
		return *reply, err
	case np.Tregister:
		reply := &np.Ropen{}
		err := s.protsrv.Register(sess, req, reply)
		return *reply, err
	case np.Tderegister:
		reply := &np.Ropen{}
		err := s.protsrv.Deregister(sess, req, reply)
		return *reply, err
	default:
		return np.ErrUnknownMsg, nil
	}
}
