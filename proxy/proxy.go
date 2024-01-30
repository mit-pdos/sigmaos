package proxy

import (
	"os/user"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/path"
	"sigmaos/pathclnt"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/serr"
	"sigmaos/sessp"
	"sigmaos/sessstatesrv"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
)

type Npd struct {
	lip  sp.Tip
	pcfg *proc.ProcEnv
	st   *sessstatesrv.SessionTable
}

func NewNpd(pcfg *proc.ProcEnv, lip sp.Tip) *Npd {
	npd := &Npd{lip, pcfg, nil}
	npd.st = sessstatesrv.NewSessionTable(npd.newProtServer, npd)
	return npd
}

func (npd *Npd) newProtServer(sesssrv sps.SessServer, sid sessp.Tsession) sps.Protsrv {
	return newNpConn(npd.pcfg, string(npd.lip))
}

func (npd *Npd) serve(fm *sessp.FcallMsg) {
	s := sessp.Tsession(fm.Fc.Session)
	sess, _ := npd.st.Lookup(s)
	msg, data, rerror, _, _ := sess.Dispatch(fm.Msg, fm.Data)
	if rerror != nil {
		msg = rerror
	}
	reply := sessp.NewFcallMsg(msg, nil, s, nil)
	reply.Data = data
	reply.Fc.Tag = fm.Fc.Tag
	sess.SendConn(reply)
}

func (npd *Npd) Register(sid sessp.Tsession, conn sps.Conn) *serr.Err {
	sess := npd.st.Alloc(sid)
	sess.SetConn(conn)
	return nil
}

// Disassociate a connection with a session, and let it close gracefully.
func (npd *Npd) Unregister(sid sessp.Tsession, conn sps.Conn) {
	// If this connection hasn't been associated with a session yet, return.
	if sid == sessp.NoSession {
		return
	}
	sess := npd.st.Alloc(sid)
	sess.UnsetConn(conn)
}

func (npd *Npd) SrvFcall(fc *sessp.FcallMsg) *serr.Err {
	go npd.serve(fc)
	return nil
}

// The connection from the kernel/client
type NpConn struct {
	mu        sync.Mutex
	principal *sp.Tprincipal
	fidc      *fidclnt.FidClnt
	pc        *pathclnt.PathClnt
	fm        *fidMap
	cid       sp.TclntId
}

func newNpConn(pcfg *proc.ProcEnv, lip string) *NpConn {
	npc := &NpConn{}
	npc.fidc = fidclnt.NewFidClnt(sp.ROOTREALM.String())
	npc.pc = pathclnt.NewPathClnt(pcfg, npc.fidc)
	npc.fm = newFidMap()
	npc.cid = sp.TclntId(rand.Uint64())
	return npc
}

func (npc *NpConn) Version(args *sp.Tversion, rets *sp.Rversion) *sp.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (npc *NpConn) Auth(args *sp.Tauth, rets *sp.Rauth) *sp.Rerror {
	return sp.NewRerrorCode(serr.TErrNotSupported)
}

func (npc *NpConn) Attach(args *sp.Tattach, rets *sp.Rattach) (sp.TclntId, *sp.Rerror) {
	u, error := user.Current()
	if error != nil {
		return sp.NoClntId, sp.NewRerrorSerr(serr.NewErrError(error))
	}
	npc.principal = &sp.Tprincipal{
		ID:       u.Uid,
		TokenStr: proc.NOT_SET,
	}

	mnt, error := npc.pc.GetNamedMount()
	if error != nil {
		db.DPrintf(db.ERROR, "Error GetNamedMount: %v", error)
		return sp.NoClntId, sp.NewRerrorSerr(serr.NewErrError(error))
	}
	fid, err := npc.fidc.Attach(npc.principal, npc.cid, mnt.Addr, "", "")
	if err != nil {
		db.DPrintf(db.PROXY, "Attach args %v err %v\n", args, err)
		return sp.NoClntId, sp.NewRerrorSerr(err)
	}
	if err := npc.pc.Mount(fid, sp.NAMED); err != nil {
		db.DPrintf(db.PROXY, "Attach args %v mount err %v\n", args, err)
		return sp.NoClntId, sp.NewRerrorSerr(serr.NewErrError(err))
	}
	rets.Qid = npc.fidc.Qid(fid)
	npc.fm.mapTo(args.Tfid(), fid)
	npc.fidc.Lookup(fid).SetPath(path.Split(sp.NAMED))
	db.DPrintf(db.PROXY, "Attach args %v rets %v fid %v\n", args, rets, fid)
	return args.TclntId(), nil
}

func (npc *NpConn) Detach(args *sp.Tdetach, rets *sp.Rdetach) *sp.Rerror {
	db.DPrintf(db.PROXY, "Detach\n")
	return nil
}

func (npc *NpConn) Walk(args *sp.Twalk, rets *sp.Rwalk) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.NewRerrorCode(serr.TErrNotfound)
	}
	fid1, err := npc.pc.Walk(fid, args.Wnames, &sp.Tprincipal{
		ID:       "proxy",
		TokenStr: proc.NOT_SET,
	})
	if err != nil {
		db.DPrintf(db.PROXY, "Walk args %v err: %v\n", args, err)
		return sp.NewRerrorSerr(err)
	}
	qids := npc.pc.Qids(fid1)
	rets.Qids = qids[len(qids)-len(args.Wnames):]
	npc.fm.mapTo(args.Tnewfid(), fid1)
	return nil
}

func (npc *NpConn) Open(args *sp.Topen, rets *sp.Ropen) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.NewRerrorCode(serr.TErrNotfound)
	}
	qid, err := npc.fidc.Open(fid, args.Tmode())
	if err != nil {
		db.DPrintf(db.PROXY, "Open args %v err: %v\n", args, err)
		return sp.NewRerrorSerr(err)
	}
	rets.Qid = qid
	db.DPrintf(db.PROXY, "Open args %v rets: %v\n", args, rets)
	return nil
}

func (npc *NpConn) Watch(args *sp.Twatch, rets *sp.Ropen) *sp.Rerror {
	return nil
}

func (npc *NpConn) Create(args *sp.Tcreate, rets *sp.Rcreate) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.NewRerrorCode(serr.TErrNotfound)
	}
	fid1, err := npc.fidc.Create(fid, args.Name, args.Tperm(), args.Tmode(), sp.NoLeaseId, sp.NoFence())
	if err != nil {
		db.DPrintf(db.PROXY, "Create args %v err: %v\n", args, err)
		return sp.NewRerrorSerr(err)
	}
	if fid != fid1 {
		db.DPrintf(db.ALWAYS, "Create fid %v fid1 %v\n", fid, fid1)
	}
	rets.Qid = npc.pc.Qid(fid1)
	db.DPrintf(db.PROXY, "Create args %v rets: %v\n", args, rets)
	return nil
}

func (npc *NpConn) Clunk(args *sp.Tclunk, rets *sp.Rclunk) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.NewRerrorCode(serr.TErrNotfound)
	}
	err := npc.fidc.Clunk(fid)
	if err != nil {
		db.DPrintf(db.PROXY, "Clunk: args %v err %v\n", args, err)
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROXY, "Clunk: args %v rets %v\n", args, rets)
	return nil
}

func (npc *NpConn) Remove(args *sp.Tremove, rets *sp.Rremove) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.NewRerrorCode(serr.TErrNotfound)
	}
	err := npc.fidc.Remove(fid, sp.NullFence())
	if err != nil {
		db.DPrintf(db.PROXY, "Remove: args %v err %v\n", args, err)
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROXY, "Remove: args %v rets %v\n", args, rets)
	return nil
}

func (npc *NpConn) RemoveFile(args *sp.Tremovefile, rets *sp.Rremove) *sp.Rerror {
	return nil
}

func (npc *NpConn) Stat(args *sp.Tstat, rets *sp.Rstat) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.NewRerrorCode(serr.TErrNotfound)
	}
	st, err := npc.fidc.Stat(fid)
	if err != nil {
		db.DPrintf(db.PROXY, "Stats: args %v err %v\n", args, err)
		return sp.NewRerrorSerr(err)
	}
	rets.Stat = st
	db.DPrintf(db.PROXY, "Stat: req %v rets %v\n", args, rets)
	return nil
}

func (npc *NpConn) Wstat(args *sp.Twstat, rets *sp.Rwstat) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.NewRerrorCode(serr.TErrNotfound)
	}
	err := npc.fidc.Wstat(fid, args.Stat, sp.NullFence())
	if err != nil {
		db.DPrintf(db.PROXY, "Wstats: args %v err %v\n", args, err)
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROXY, "Wstat: req %v rets %v\n", args, rets)
	return nil
}

func (npc *NpConn) Renameat(args *sp.Trenameat, rets *sp.Rrenameat) *sp.Rerror {
	return sp.NewRerrorCode(serr.TErrNotSupported)
}

func (npc *NpConn) ReadF(args *sp.TreadF, rets *sp.Rread) ([]byte, *sp.Rerror) {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return nil, sp.NewRerrorCode(serr.TErrNotfound)
	}
	d, err := npc.fidc.ReadF(fid, args.Toffset(), args.Tcount(), sp.NullFence())
	if err != nil {
		db.DPrintf(db.PROXY, "Read: args %v err %v\n", args, err)
		return nil, sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROXY, "ReadUV: args %v rets %v %d\n", args, rets, len(d))
	qid := npc.pc.Qid(fid)
	if sp.Qtype(qid.Type)&sp.QTDIR == sp.QTDIR {
		d1, err1 := Sp2NpDir(d, args.Tcount())
		if err != nil {
			db.DPrintf(db.PROXY, "Read: Sp2NpDir err %v\n", err1)
			return nil, sp.NewRerrorSerr(serr.NewErrError(err1))
		}
		d = d1
	}
	db.DPrintf(db.PROXY, "Read: args %v rets %v %v\n", args, rets, len(d))
	return d, nil
}

func (npc *NpConn) WriteF(args *sp.TwriteF, data []byte, rets *sp.Rwrite) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.NewRerrorCode(serr.TErrNotfound)
	}
	n, err := npc.fidc.WriteF(fid, args.Toffset(), data, sp.NullFence())
	if err != nil {
		db.DPrintf(db.PROXY, "Write: args %v err %v\n", args, err)
		return sp.NewRerrorSerr(err)
	}
	rets.Count = uint32(n)
	db.DPrintf(db.PROXY, "Write: args %v rets %v\n", args, rets)
	return nil
}

func (npc *NpConn) GetFile(args *sp.Tgetfile, rets *sp.Rread) ([]byte, *sp.Rerror) {
	return nil, nil
}

func (npc *NpConn) PutFile(args *sp.Tputfile, d []byte, rets *sp.Rwrite) *sp.Rerror {
	return nil
}

func (npc *NpConn) WriteRead(args *sp.Twriteread, d []byte, rets *sp.Rread) ([]byte, *sp.Rerror) {
	return nil, nil
}
