package proxy

import (
	"log"
	"net"

	np "ulambda/ninep"
	"ulambda/npclnt"
	"ulambda/npsrv"
)

type NpConn struct {
	conn net.Conn
	clnt *npclnt.NpClnt
	npc  *npclnt.NpChan
}

func makeNpConn(conn net.Conn) *NpConn {
	npc := &NpConn{}
	npc.clnt = npclnt.MakeNpClnt(true)
	npc.conn = conn
	npc.npc = npc.clnt.MakeNpChan(":1111")
	return npc
}

type Npd struct {
}

func MakeNpd() *Npd {
	npd := &Npd{}
	return npd
}

func (npd *Npd) Connect(conn net.Conn) npsrv.NpAPI {
	clnt := makeNpConn(conn)
	return clnt
}

func (npc *NpConn) Version(args np.Tversion, rets *np.Rversion) *np.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (npc *NpConn) Auth(args np.Tauth, rets *np.Rauth) *np.Rerror {
	return np.ErrUnknownMsg
}

func (npc *NpConn) Attach(args np.Tattach, rets *np.Rattach) *np.Rerror {
	log.Print("received attach")
	reply, err := npc.clnt.Attach(":1111", args.Fid, np.Split(args.Aname))
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	rets.Qid = reply.Qid
	return nil
}

func (npc *NpConn) Walk(args np.Twalk, rets *np.Rwalk) *np.Rerror {
	reply, err := npc.npc.Walk(args.Fid, args.NewFid, args.Wnames)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	*rets = *reply
	return nil
}

func (npc *NpConn) Open(args np.Topen, rets *np.Ropen) *np.Rerror {
	reply, err := npc.npc.Open(args.Fid, args.Mode)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	*rets = *reply
	return nil
}

func (npc *NpConn) Create(args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	reply, err := npc.npc.Create(args.Fid, args.Name, args.Perm, args.Mode)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	*rets = *reply
	return nil
}

func (npc *NpConn) Clunk(args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	err := npc.npc.Clunk(args.Fid)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	return nil
}

func (npc *NpConn) Flush(args np.Tflush, rets *np.Rflush) *np.Rerror {
	err := npc.npc.Flush(args.Oldtag)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	return nil
}

func (npc *NpConn) Read(args np.Tread, rets *np.Rread) *np.Rerror {
	reply, err := npc.npc.Read(args.Fid, args.Offset, args.Count)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	*rets = *reply
	return nil
}

func (npc *NpConn) Write(args np.Twrite, rets *np.Rwrite) *np.Rerror {
	reply, err := npc.npc.Write(args.Fid, args.Offset, args.Data)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	*rets = *reply
	return nil
}

func (npc *NpConn) Remove(args np.Tremove, rets *np.Rremove) *np.Rerror {
	err := npc.npc.Remove(args.Fid)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	return nil
}

func (npc *NpConn) Stat(args np.Tstat, rets *np.Rstat) *np.Rerror {
	reply, err := npc.npc.Stat(args.Fid)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	*rets = *reply
	return nil
}

func (npc *NpConn) Wstat(args np.Twstat, rets *np.Rwstat) *np.Rerror {
	reply, err := npc.npc.Wstat(args.Fid, &args.Stat)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	*rets = *reply
	return nil
}

func (npc *NpConn) Pipe(args np.Tmkpipe, rets *np.Rmkpipe) *np.Rerror {
	return &np.Rerror{"Not supported"}
}
