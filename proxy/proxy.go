package proxy

import (
	"log"
	"net"

	np "ulambda/ninep"
	"ulambda/npclnt"
	"ulambda/npsrv"
)

const MAXSYMLINK = 20

// The connection from the kernel/client
type NpConn struct {
	conn net.Conn
	clnt *npclnt.NpClnt
	fids map[np.Tfid]*npclnt.NpChan // The outgoing channels to servers proxied

}

func makeNpConn(conn net.Conn) *NpConn {
	npc := &NpConn{}
	npc.conn = conn
	npc.clnt = npclnt.MakeNpClnt(true)
	npc.fids = make(map[np.Tfid]*npclnt.NpChan)
	return npc
}

type Npd struct {
}

func MakeNpd() *Npd {
	return &Npd{}
}

// XXX should/is happen only once for the one mount for :1110
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
	reply, err := npc.clnt.Attach(":1111", args.Fid, np.Split(args.Aname))
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	npc.fids[args.Fid] = npc.clnt.MakeNpChan(":1111")
	rets.Qid = reply.Qid
	return nil
}

func (npc *NpConn) npch(fid np.Tfid) *npclnt.NpChan {
	ch, ok := npc.fids[fid]
	if !ok {
		log.Fatal("npch: unknown fid ", fid)
	}
	return ch
}

func (npc *NpConn) Walk(args np.Twalk, rets *np.Rwalk) *np.Rerror {
	for i := 0; i < MAXSYMLINK; i++ {
		reply, err := npc.npch(args.Fid).Walk(args.Fid, args.NewFid, args.Wnames)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		if len(reply.Qids) == 0 { // clone args.Fid?
			npc.fids[args.NewFid] = npc.npch(args.Fid)
			*rets = *reply
			break
		}
		qid := reply.Qids[len(reply.Qids)-1]
		if qid.Type == np.QTSYMLINK {
			log.Print("symlink")
			return nil
		} else { // newFid is at same server as args.Fid
			npc.fids[args.NewFid] = npc.npch(args.Fid)
			*rets = *reply
			break
		}
	}
	return nil
}

func (npc *NpConn) Open(args np.Topen, rets *np.Ropen) *np.Rerror {
	reply, err := npc.npch(args.Fid).Open(args.Fid, args.Mode)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	*rets = *reply
	return nil
}

func (npc *NpConn) Create(args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	reply, err := npc.npch(args.Fid).Create(args.Fid, args.Name, args.Perm, args.Mode)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	*rets = *reply
	return nil
}

func (npc *NpConn) Clunk(args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	err := npc.npch(args.Fid).Clunk(args.Fid)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	delete(npc.fids, args.Fid)
	return nil
}

func (npc *NpConn) Flush(args np.Tflush, rets *np.Rflush) *np.Rerror {
	return nil
}

func (npc *NpConn) Read(args np.Tread, rets *np.Rread) *np.Rerror {
	reply, err := npc.npch(args.Fid).Read(args.Fid, args.Offset, args.Count)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	*rets = *reply
	return nil
}

func (npc *NpConn) Write(args np.Twrite, rets *np.Rwrite) *np.Rerror {
	reply, err := npc.npch(args.Fid).Write(args.Fid, args.Offset, args.Data)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	*rets = *reply
	return nil
}

func (npc *NpConn) Remove(args np.Tremove, rets *np.Rremove) *np.Rerror {
	err := npc.npch(args.Fid).Remove(args.Fid)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	return nil
}

func (npc *NpConn) Stat(args np.Tstat, rets *np.Rstat) *np.Rerror {
	reply, err := npc.npch(args.Fid).Stat(args.Fid)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	*rets = *reply
	return nil
}

func (npc *NpConn) Wstat(args np.Twstat, rets *np.Rwstat) *np.Rerror {
	reply, err := npc.npch(args.Fid).Wstat(args.Fid, &args.Stat)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	*rets = *reply
	return nil
}

func (npc *NpConn) Pipe(args np.Tmkpipe, rets *np.Rmkpipe) *np.Rerror {
	return &np.Rerror{"Not supported"}
}
