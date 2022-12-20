package proxy

import (
	"os/user"
	"sync"

	db "sigmaos/debug"
	"sigmaos/sessp"
	"sigmaos/fidclnt"
	"sigmaos/fslib"
	"sigmaos/path"
	"sigmaos/pathclnt"
	"sigmaos/protclnt"
	"sigmaos/sessstatesrv"
	sp "sigmaos/sigmap"
	"sigmaos/threadmgr"
)

type Npd struct {
	named []string
	st    *sessstatesrv.SessionTable
}

func MakeNpd() *Npd {
	npd := &Npd{fslib.Named(), nil}
	tm := threadmgr.MakeThreadMgrTable(nil, false)
	npd.st = sessstatesrv.MakeSessionTable(npd.mkProtServer, npd, tm)
	return npd
}

func (npd *Npd) mkProtServer(sesssrv sp.SessServer, sid sessp.Tsession) sp.Protsrv {
	return makeNpConn(npd.named)
}

func (npd *Npd) serve(fm *sessp.FcallMsg) {
	s := sessp.Tsession(fm.Fc.Session)
	sess, _ := npd.st.Lookup(s)
	msg, data, _, rerror := sess.Dispatch(fm.Msg, fm.Data)
	if rerror != nil {
		msg = rerror
	}
	reply := sessp.MakeFcallMsg(msg, nil, sessp.Tclient(fm.Fc.Client), s, nil, nil, sessp.MakeFenceNull())
	reply.Data = data
	reply.Fc.Tag = fm.Fc.Tag
	sess.SendConn(reply)
}

func (npd *Npd) Register(cid sessp.Tclient, sid sessp.Tsession, conn sp.Conn) *sessp.Err {
	sess := npd.st.Alloc(cid, sid)
	sess.SetConn(conn)
	return nil
}

// Disassociate a connection with a session, and let it close gracefully.
func (npd *Npd) Unregister(cid sessp.Tclient, sid sessp.Tsession, conn sp.Conn) {
	// If this connection hasn't been associated with a session yet, return.
	if sid == sessp.NoSession {
		return
	}
	sess := npd.st.Alloc(cid, sid)
	sess.UnsetConn(conn)
}

func (npd *Npd) SrvFcall(fc *sessp.FcallMsg) {
	go npd.serve(fc)
}

func (npd *Npd) Snapshot() []byte {
	return nil
}

func (npd *Npd) Restore(b []byte) {
}

// The connection from the kernel/client
type NpConn struct {
	mu    sync.Mutex
	clnt  *protclnt.Clnt
	uname string
	fidc  *fidclnt.FidClnt
	pc    *pathclnt.PathClnt
	named []string
	fm    *fidMap
}

func makeNpConn(named []string) *NpConn {
	npc := &NpConn{}
	npc.clnt = protclnt.MakeClnt()
	npc.named = named
	npc.fidc = fidclnt.MakeFidClnt()
	npc.pc = pathclnt.MakePathClnt(npc.fidc, sessp.Tsize(1_000_000))
	npc.fm = mkFidMap()
	return npc
}

func (npc *NpConn) Version(args *sp.Tversion, rets *sp.Rversion) *sp.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (npc *NpConn) Auth(args *sp.Tauth, rets *sp.Rauth) *sp.Rerror {
	return sp.MkRerrorCode(sessp.TErrNotSupported)
}

func (npc *NpConn) Attach(args *sp.Tattach, rets *sp.Rattach) *sp.Rerror {
	u, error := user.Current()
	if error != nil {
		return sp.MkRerror(sessp.MkErrError(error))
	}
	npc.uname = u.Uid

	fid, err := npc.fidc.Attach(npc.uname, npc.named, "", "")
	if err != nil {
		db.DPrintf(db.PROXY, "Attach args %v err %v\n", args, err)
		return sp.MkRerror(err)
	}
	if err := npc.pc.Mount(fid, sp.NAMED); err != nil {
		db.DPrintf(db.PROXY, "Attach args %v mount err %v\n", args, err)
		return sp.MkRerror(sessp.MkErrError(err))
	}
	rets.Qid = npc.fidc.Qid(fid)
	npc.fm.mapTo(args.Tfid(), fid)
	npc.fidc.Lookup(fid).SetPath(path.Split(sp.NAMED))
	db.DPrintf(db.PROXY, "Attach args %v rets %v fid %v\n", args, rets, fid)
	return nil
}

func (npc *NpConn) Detach(rets *sp.Rdetach, detach sp.DetachF) *sp.Rerror {
	db.DPrintf(db.PROXY, "Detach\n")
	return nil
}

func (npc *NpConn) Walk(args *sp.Twalk, rets *sp.Rwalk) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.MkRerrorCode(sessp.TErrNotfound)
	}
	fid1, err := npc.pc.Walk(fid, args.Wnames)
	if err != nil {
		db.DPrintf(db.PROXY, "Walk args %v err: %v\n", args, err)
		return sp.MkRerror(err)
	}
	qids := npc.pc.Qids(fid1)
	rets.Qids = qids[len(qids)-len(args.Wnames):]
	npc.fm.mapTo(args.Tnewfid(), fid1)
	return nil
}

func (npc *NpConn) Open(args *sp.Topen, rets *sp.Ropen) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.MkRerrorCode(sessp.TErrNotfound)
	}
	qid, err := npc.fidc.Open(fid, args.Tmode())
	if err != nil {
		db.DPrintf(db.PROXY, "Open args %v err: %v\n", args, err)
		return sp.MkRerror(err)
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
		return sp.MkRerrorCode(sessp.TErrNotfound)
	}
	fid1, err := npc.fidc.Create(fid, args.Name, args.Tperm(), args.Tmode())
	if err != nil {
		db.DPrintf(db.PROXY, "Create args %v err: %v\n", args, err)
		return sp.MkRerror(err)
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
		return sp.MkRerrorCode(sessp.TErrNotfound)
	}
	err := npc.fidc.Clunk(fid)
	if err != nil {
		db.DPrintf(db.PROXY, "Clunk: args %v err %v\n", args, err)
		return sp.MkRerror(err)
	}
	db.DPrintf(db.PROXY, "Clunk: args %v rets %v\n", args, rets)
	return nil
}

func (npc *NpConn) Remove(args *sp.Tremove, rets *sp.Rremove) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.MkRerrorCode(sessp.TErrNotfound)
	}
	err := npc.fidc.Remove(fid)
	if err != nil {
		db.DPrintf(db.PROXY, "Remove: args %v err %v\n", args, err)
		return sp.MkRerror(err)
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
		return sp.MkRerrorCode(sessp.TErrNotfound)
	}
	st, err := npc.fidc.Stat(fid)
	if err != nil {
		db.DPrintf(db.PROXY, "Stats: args %v err %v\n", args, err)
		return sp.MkRerror(err)
	}
	rets.Stat = st
	db.DPrintf(db.PROXY, "Stat: req %v rets %v\n", args, rets)
	return nil
}

func (npc *NpConn) Wstat(args *sp.Twstat, rets *sp.Rwstat) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.MkRerrorCode(sessp.TErrNotfound)
	}
	err := npc.fidc.Wstat(fid, args.Stat)
	if err != nil {
		db.DPrintf(db.PROXY, "Wstats: args %v err %v\n", args, err)
		return sp.MkRerror(err)
	}
	db.DPrintf(db.PROXY, "Wstat: req %v rets %v\n", args, rets)
	return nil
}

func (npc *NpConn) Renameat(args *sp.Trenameat, rets *sp.Rrenameat) *sp.Rerror {
	return sp.MkRerrorCode(sessp.TErrNotSupported)
}

func (npc *NpConn) ReadV(args *sp.TreadV, rets *sp.Rread) ([]byte, *sp.Rerror) {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return nil, sp.MkRerrorCode(sessp.TErrNotfound)
	}
	d, err := npc.fidc.ReadVU(fid, args.Toffset(), args.Tcount(), sp.NoV)
	if err != nil {
		db.DPrintf(db.PROXY, "Read: args %v err %v\n", args, err)
		return nil, sp.MkRerror(err)
	}
	db.DPrintf(db.PROXY, "ReadUV: args %v rets %v\n", args, rets)
	qid := npc.pc.Qid(fid)
	if sp.Qtype(qid.Type)&sp.QTDIR == sp.QTDIR {
		d, err = Sp2NpDir(d, args.Tcount())
		if err != nil {
			return nil, sp.MkRerror(err)
		}
	}
	db.DPrintf(db.PROXY, "Read: args %v rets %v\n", args, rets)
	return d, nil
}

func (npc *NpConn) WriteV(args *sp.TwriteV, data []byte, rets *sp.Rwrite) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.MkRerrorCode(sessp.TErrNotfound)
	}
	n, err := npc.fidc.WriteV(fid, args.Toffset(), data, sp.NoV)
	if err != nil {
		db.DPrintf(db.PROXY, "Write: args %v err %v\n", args, err)
		return sp.MkRerror(err)
	}
	rets.Count = uint32(n)
	db.DPrintf(db.PROXY, "Write: args %v rets %v\n", args, rets)
	return nil
}

func (npc *NpConn) GetFile(args *sp.Tgetfile, rets *sp.Rread) ([]byte, *sp.Rerror) {
	return nil, nil
}

func (npc *NpConn) SetFile(args *sp.Tsetfile, d []byte, rets *sp.Rwrite) *sp.Rerror {
	return nil
}

func (npc *NpConn) PutFile(args *sp.Tputfile, d []byte, rets *sp.Rwrite) *sp.Rerror {
	return nil
}

func (npc *NpConn) WriteRead(args *sp.Twriteread, d []byte, rets *sp.Rread) ([]byte, *sp.Rerror) {
	return nil, nil
}

func (npc *NpConn) Snapshot() []byte {
	return nil
}
