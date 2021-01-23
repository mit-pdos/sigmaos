package proxy

import (
	"log"
	"net"
	"strings"
	"sync"

	np "ulambda/ninep"
	"ulambda/npclnt"
	"ulambda/npsrv"
)

const MAXSYMLINK = 20

// The connection from the kernel/client
type NpConn struct {
	mu   sync.Mutex
	conn net.Conn
	clnt *npclnt.NpClnt
	fids map[np.Tfid]*npclnt.NpChan // The outgoing channels to servers proxied

}

func makeNpConn(conn net.Conn) *NpConn {
	npc := &NpConn{}
	npc.conn = conn
	npc.clnt = npclnt.MakeNpClnt(false)
	npc.fids = make(map[np.Tfid]*npclnt.NpChan)
	return npc
}

func (npc *NpConn) npch(fid np.Tfid) *npclnt.NpChan {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	ch, ok := npc.fids[fid]
	if !ok {
		log.Fatal("npch: unknown fid ", fid)
	}
	return ch
}

func (npc *NpConn) addch(fid np.Tfid, ch *npclnt.NpChan) {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	npc.fids[fid] = ch
}

func (npc *NpConn) delch(fid np.Tfid) {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	delete(npc.fids, fid)
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
	npc.addch(args.Fid, npc.clnt.MakeNpChan(":1111"))
	rets.Qid = reply.Qid
	return nil
}

// XXX avoid duplication with fsclnt
func isRemoteTarget(target string) bool {
	return strings.Contains(target, ":")
}

// XXX avoid duplication with fsclnt
func splitTarget(target string) (string, string) {
	parts := strings.Split(target, ":")
	server := parts[0] + ":" + parts[1] + ":" + parts[2] + ":" + parts[3]
	return server, parts[len(parts)-1]
}

// XXX avoid duplication with fsclnt
func (npc *NpConn) autoMount(newfid np.Tfid, target string, path []string) (np.Tqid, error) {
	log.Printf("automount %v to %v\n", target, path)
	server, _ := splitTarget(target)
	reply, err := npc.clnt.Attach(server, newfid, path)
	if err != nil {
		return np.Tqid{}, err
	}
	npc.addch(newfid, npc.clnt.MakeNpChan(server))
	return reply.Qid, nil
}

// XXX avoid duplication with fsclnt
func (npc *NpConn) readLink(fid np.Tfid) (string, error) {
	_, err := npc.npch(fid).Open(fid, np.OREAD)
	if err != nil {
		return "", err
	}
	reply, err := npc.npch(fid).Read(fid, 0, 1024)
	if err != nil {
		return "", err
	}
	// XXX clunk fid
	npc.delch(fid)
	return string(reply.Data), nil
}

func (npc *NpConn) Walk(args np.Twalk, rets *np.Rwalk) *np.Rerror {
	path := args.Wnames
	// XXX accumulate qids
	for i := 0; i < MAXSYMLINK; i++ {
		reply, err := npc.npch(args.Fid).Walk(args.Fid, args.NewFid, path)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		if len(reply.Qids) == 0 { // clone args.Fid?
			npc.addch(args.NewFid, npc.npch(args.Fid))
			*rets = *reply
			break
		}
		qid := reply.Qids[len(reply.Qids)-1]
		if qid.Type == np.QTSYMLINK {
			todo := len(path) - len(reply.Qids)
			log.Print("symlink ", todo, path)

			// args.Newfid is fid for symlink
			npc.addch(args.NewFid, npc.npch(args.Fid))

			target, err := npc.readLink(args.NewFid)
			if err != nil {
				return np.ErrUnknownfid
			}
			// XXX assumes symlink is final component of walk
			if isRemoteTarget(target) {
				qid, err = npc.autoMount(args.NewFid, target, path[todo:])
				if err != nil {
					return np.ErrUnknownfid
				}
				reply.Qids[len(reply.Qids)-1] = qid
				path = path[todo:]
				log.Printf("automounted: %v -> %v, %v\n", args.NewFid,
					target, path)
				*rets = *reply
				break
			} else {
				log.Fatal("don't handle")
			}
		} else { // newFid is at same server as args.Fid
			npc.addch(args.NewFid, npc.npch(args.Fid))
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
	npc.delch(args.Fid)
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
