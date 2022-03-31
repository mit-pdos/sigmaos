package proxy

import (
	"os/user"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/pathclnt"
	"ulambda/protclnt"
	"ulambda/session"
	"ulambda/threadmgr"
)

type Npd struct {
	named []string
	st    *session.SessionTable
}

func MakeNpd() *Npd {
	npd := &Npd{fslib.Named(), nil}
	tm := threadmgr.MakeThreadMgrTable(nil, false)
	npd.st = session.MakeSessionTable(npd.mkProtServer, npd, tm)
	return npd
}

func (npd *Npd) mkProtServer(fssrv np.FsServer, sid np.Tsession) np.Protsrv {
	return makeNpConn(npd.named)
}

func (npd *Npd) serve(fc *np.Fcall) {
	t := fc.Tag
	sess, _ := npd.st.Lookup(fc.Session)
	reply, _, rerror := sess.Dispatch(fc.Msg)
	if rerror != nil {
		reply = *rerror
	}
	fcall := np.MakeFcall(reply, 0, nil, np.NoFence)
	fcall.Tag = t
	sess.SendConn(fcall)
}

func (npd *Npd) Register(sid np.Tsession, conn *np.Conn) *np.Err {
	sess := npd.st.Alloc(sid)
	sess.SetConn(conn)
	return nil
}

func (npd *Npd) SrvFcall(fcall *np.Fcall) {
	go npd.serve(fcall)
}

func (npd *Npd) Snapshot() []byte {
	return nil
}

func (npd *Npd) Restore(b []byte) {
}

//
// XXX convert to use npobjsrv, fidclnt, pathclnt
//

const MAXSYMLINK = 20

// The connection from the kernel/client
type NpConn struct {
	mu    sync.Mutex
	clnt  *protclnt.Clnt
	uname string
	fids  map[np.Tfid]*protclnt.ProtClnt // The outgoing channels to servers proxied
	named []string
}

func makeNpConn(named []string) *NpConn {
	npc := &NpConn{}
	npc.clnt = protclnt.MakeClnt()
	npc.fids = make(map[np.Tfid]*protclnt.ProtClnt)
	npc.named = named
	return npc
}

func (npc *NpConn) npch(fid np.Tfid) *protclnt.ProtClnt {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	ch, ok := npc.fids[fid]
	if !ok {
		db.DFatalf("npch: unknown fid %v", fid)
	}
	return ch
}

func (npc *NpConn) addch(fid np.Tfid, ch *protclnt.ProtClnt) {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	npc.fids[fid] = ch
}

func (npc *NpConn) delch(fid np.Tfid) {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	delete(npc.fids, fid)
}

func (npc *NpConn) Version(args np.Tversion, rets *np.Rversion) *np.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (npc *NpConn) Auth(args np.Tauth, rets *np.Rauth) *np.Rerror {
	return np.MkErr(np.TErrNotSupported, "Auth").Rerror()
}

func (npc *NpConn) Attach(args np.Tattach, rets *np.Rattach) *np.Rerror {
	u, error := user.Current()
	if error != nil {
		return &np.Rerror{error.Error()}
	}
	npc.uname = u.Uid

	reply, err := npc.clnt.Attach(npc.named, npc.uname, args.Fid, []string{""})
	if err != nil {
		return err.Rerror()
	}
	npc.addch(args.Fid, npc.clnt.MakeProtClnt(npc.named))
	rets.Qid = reply.Qid
	return nil
}

func (npc *NpConn) Detach(rets *np.Rdetach) *np.Rerror {
	db.DPrintf("9POBJ", "Detach\n")
	return nil
}

// XXX avoid duplication with fidclnt
func (npc *NpConn) autoMount(newfid np.Tfid, target string, path []string) (np.Tqid, error) {
	db.DPrintf("PROXY", "automount %v to %v\n", target, path)
	server, _ := pathclnt.SplitTarget(target)
	reply, err := npc.clnt.Attach([]string{server}, npc.uname, newfid, []string{""})
	if err != nil {
		return np.Tqid{}, err
	}
	npc.addch(newfid, npc.clnt.MakeProtClnt([]string{server}))
	return reply.Qid, nil
}

// XXX avoid duplication with fidclnt
func (npc *NpConn) readLink(fid np.Tfid) (string, error) {
	_, err := npc.npch(fid).Open(fid, np.OREAD)
	if err != nil {
		return "", err
	}
	reply, err := npc.npch(fid).Read(fid, 0, 1024)
	if err != nil {
		return "", err
	}
	npc.delch(fid)
	return string(reply.Data), nil
}

func (npc *NpConn) Walk(args np.Twalk, rets *np.Rwalk) *np.Rerror {
	path := args.Wnames
	// XXX accumulate qids
	for i := 0; i < MAXSYMLINK; i++ {
		reply, err := npc.npch(args.Fid).Walk(args.Fid, args.NewFid, path)
		if err != nil {
			return err.Rerror()
		}
		if len(reply.Qids) == 0 { // clone args.Fid?
			npc.addch(args.NewFid, npc.npch(args.Fid))
			*rets = *reply
			break
		}
		qid := reply.Qids[len(reply.Qids)-1]
		if qid.Type&np.QTSYMLINK == np.QTSYMLINK {
			todo := len(path) - len(reply.Qids)
			db.DPrintf("PROXY", "symlink %v %v\n", todo, path)

			// args.Newfid is fid for symlink
			npc.addch(args.NewFid, npc.npch(args.Fid))

			target, err := npc.readLink(args.NewFid)
			if err != nil {
				return np.MkErr(np.TErrUnknownfid, path).Rerror()
			}
			// XXX assumes symlink is final component of walk
			if pathclnt.IsRemoteTarget(target) {
				qid, err = npc.autoMount(args.NewFid, target, path[todo:])
				if err != nil {
					return np.MkErr(np.TErrUnknownfid, path).Rerror()
				}
				reply.Qids[len(reply.Qids)-1] = qid
				path = path[todo:]
				db.DPrintf("PROXY", "automounted: %v -> %v, %v\n", args.NewFid,
					target, path)
				*rets = *reply
				break
			} else {
				db.DFatalf("don't handle")
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

func (npc *NpConn) Watch(args np.Twatch, rets *np.Ropen) *np.Rerror {
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

func (npc *NpConn) RemoveFile(args np.Tremovefile, rets *np.Rremove) *np.Rerror {
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

func (npc *NpConn) Renameat(args np.Trenameat, rets *np.Rrenameat) *np.Rerror {
	return np.MkErr(np.TErrNotSupported, args).Rerror()
}

func (npc *NpConn) ReadV(args np.TreadV, rets *np.Rread) *np.Rerror {
	return np.MkErr(np.TErrNotSupported, args).Rerror()
}

func (npc *NpConn) WriteV(args np.TwriteV, rets *np.Rwrite) *np.Rerror {
	return np.MkErr(np.TErrNotSupported, args).Rerror()
}

func (npc *NpConn) GetFile(args np.Tgetfile, rets *np.Rgetfile) *np.Rerror {
	return nil
}

func (npc *NpConn) SetFile(args np.Tsetfile, rets *np.Rwrite) *np.Rerror {
	return nil
}

func (npc *NpConn) PutFile(args np.Tputfile, rets *np.Rwrite) *np.Rerror {
	return nil
}

func (npc *NpConn) Snapshot() []byte {
	return nil
}
