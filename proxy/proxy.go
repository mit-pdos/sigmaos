package proxy

import (
	"os/user"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fcall"
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

func (npd *Npd) mkProtServer(sesssrv sp.SessServer, sid fcall.Tsession) sp.Protsrv {
	return makeNpConn(npd.named)
}

func (npd *Npd) serve(fm *sp.FcallMsg) {
	s := fcall.Tsession(fm.Fc.Session)
	sess, _ := npd.st.Lookup(s)
	reply, _, rerror := sess.Dispatch(fm.Msg)
	if rerror != nil {
		reply = rerror
	}
	fm1 := sp.MakeFcallMsg(reply, fcall.Tclient(fm.Fc.Client), s, nil, nil, sp.MakeFenceNull())
	fm1.Fc.Tag = fm.Fc.Tag
	sess.SendConn(fm1)
}

func (npd *Npd) Register(cid fcall.Tclient, sid fcall.Tsession, conn sp.Conn) *fcall.Err {
	sess := npd.st.Alloc(cid, sid)
	sess.SetConn(conn)
	return nil
}

// Disassociate a connection with a session, and let it close gracefully.
func (npd *Npd) Unregister(cid fcall.Tclient, sid fcall.Tsession, conn sp.Conn) {
	// If this connection hasn't been associated with a session yet, return.
	if sid == fcall.NoSession {
		return
	}
	sess := npd.st.Alloc(cid, sid)
	sess.UnsetConn(conn)
}

func (npd *Npd) SrvFcall(fcall fcall.Fcall) {
	fm := fcall.(*sp.FcallMsg)
	go npd.serve(fm)
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
	npc.pc = pathclnt.MakePathClnt(npc.fidc, sp.Tsize(1_000_000))
	npc.fm = mkFidMap()
	return npc
}

// Make Wire-compatible Rerror
func MkRerrorWC(ec fcall.Terror) *sp.Rerror {
	return &sp.Rerror{ec.String()}
}

func (npc *NpConn) Version(args *sp.Tversion, rets *sp.Rversion) *sp.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (npc *NpConn) Auth(args *sp.Tauth, rets *sp.Rauth) *sp.Rerror {
	return MkRerrorWC(fcall.TErrNotSupported)
}

func (npc *NpConn) Attach(args *sp.Tattach, rets *sp.Rattach) *sp.Rerror {
	u, error := user.Current()
	if error != nil {
		return &sp.Rerror{error.Error()}
	}
	npc.uname = u.Uid

	fid, err := npc.fidc.Attach(npc.uname, npc.named, "", "")
	if err != nil {
		db.DPrintf("PROXY", "Attach args %v err %v\n", args, err)
		return sp.MkRerror(err)
	}
	if err := npc.pc.Mount(fid, sp.NAMED); err != nil {
		db.DPrintf("PROXY", "Attach args %v mount err %v\n", args, err)
		return &sp.Rerror{error.Error()}
	}
	rets.Qid = npc.fidc.Qid(fid)
	npc.fm.mapTo(args.Fid, fid)
	npc.fidc.Lookup(fid).SetPath(path.Split(sp.NAMED))
	db.DPrintf("PROXY", "Attach args %v rets %v fid %v\n", args, rets, fid)
	return nil
}

func (npc *NpConn) Detach(rets *sp.Rdetach, detach sp.DetachF) *sp.Rerror {
	db.DPrintf("PROXY", "Detach\n")
	return nil
}

func (npc *NpConn) Walk(args *sp.Twalk, rets *sp.Rwalk) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return MkRerrorWC(fcall.TErrNotfound)
	}
	fid1, err := npc.pc.Walk(fid, args.Wnames)
	if err != nil {
		db.DPrintf("PROXY", "Walk args %v err: %v\n", args, err)
		return MkRerrorWC(err.Code())
	}
	qids := npc.pc.Qids(fid1)
	rets.Qids = qids[len(qids)-len(args.Wnames):]
	npc.fm.mapTo(args.Tnewfid(), fid1)
	return nil
}

func (npc *NpConn) Open(args *sp.Topen, rets *sp.Ropen) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Fid)
	if !ok {
		return MkRerrorWC(fcall.TErrNotfound)
	}
	qid, err := npc.fidc.Open(fid, args.Mode)
	if err != nil {
		db.DPrintf("PROXY", "Open args %v err: %v\n", args, err)
		return MkRerrorWC(err.Code())
	}
	rets.Qid = qid
	db.DPrintf("PROXY", "Open args %v rets: %v\n", args, rets)
	return nil
}

func (npc *NpConn) Watch(args *sp.Twatch, rets *sp.Ropen) *sp.Rerror {
	return nil
}

func (npc *NpConn) Create(args *sp.Tcreate, rets *sp.Rcreate) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Fid)
	if !ok {
		return MkRerrorWC(fcall.TErrNotfound)
	}
	fid1, err := npc.fidc.Create(fid, args.Name, args.Perm, args.Mode)
	if err != nil {
		db.DPrintf("PROXY", "Create args %v err: %v\n", args, err)
		return MkRerrorWC(err.Code())
	}
	if fid != fid1 {
		db.DPrintf(db.ALWAYS, "Create fid %v fid1 %v\n", fid, fid1)
	}
	rets.Qid = *npc.pc.Qid(fid1)
	db.DPrintf("PROXY", "Create args %v rets: %v\n", args, rets)
	return nil
}

func (npc *NpConn) Clunk(args *sp.Tclunk, rets *sp.Rclunk) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Fid)
	if !ok {
		return MkRerrorWC(fcall.TErrNotfound)
	}
	err := npc.fidc.Clunk(fid)
	if err != nil {
		db.DPrintf("PROXY", "Clunk: args %v err %v\n", args, err)
		return MkRerrorWC(err.Code())
	}
	db.DPrintf("PROXY", "Clunk: args %v rets %v\n", args, rets)
	return nil
}

func (npc *NpConn) Flush(args *sp.Tflush, rets *sp.Rflush) *sp.Rerror {
	return nil
}

func (npc *NpConn) Read(args *sp.Tread, rets *sp.Rread) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Fid)
	if !ok {
		return MkRerrorWC(fcall.TErrNotfound)
	}
	d, err := npc.fidc.ReadVU(fid, args.Offset, args.Count, sp.NoV)
	if err != nil {
		db.DPrintf("PROXY", "Read: args %v err %v\n", args, err)
		return MkRerrorWC(err.Code())
	}
	db.DPrintf("PROXY", "ReadUV: args %v rets %v\n", args, rets)
	qid := npc.pc.Qid(fid)
	if sp.Qtype(qid.Type)&sp.QTDIR == sp.QTDIR {
		d, err = Sp2NpDir(d, args.Count)
		if err != nil {
			return MkRerrorWC(err.Code())
		}
	}
	rets.Data = d
	db.DPrintf("PROXY", "Read: args %v rets %v\n", args, rets)
	return nil
}

func (npc *NpConn) Write(args *sp.Twrite, rets *sp.Rwrite) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Fid)
	if !ok {
		return MkRerrorWC(fcall.TErrNotfound)
	}
	n, err := npc.fidc.WriteV(fid, args.Offset, args.Data, sp.NoV)
	if err != nil {
		db.DPrintf("PROXY", "Write: args %v err %v\n", args, err)
		return MkRerrorWC(err.Code())
	}
	rets.Count = n
	db.DPrintf("PROXY", "Write: args %v rets %v\n", args, rets)
	return nil
}

func (npc *NpConn) Remove(args *sp.Tremove, rets *sp.Rremove) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Fid)
	if !ok {
		return MkRerrorWC(fcall.TErrNotfound)
	}
	err := npc.fidc.Remove(fid)
	if err != nil {
		db.DPrintf("PROXY", "Remove: args %v err %v\n", args, err)
		return MkRerrorWC(err.Code())
	}
	db.DPrintf("PROXY", "Remove: args %v rets %v\n", args, rets)
	return nil
}

func (npc *NpConn) RemoveFile(args *sp.Tremovefile, rets *sp.Rremove) *sp.Rerror {
	return nil
}

func (npc *NpConn) Stat(args *sp.Tstat, rets *sp.Rstat) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Fid)
	if !ok {
		return MkRerrorWC(fcall.TErrNotfound)
	}
	st, err := npc.fidc.Stat(fid)
	if err != nil {
		db.DPrintf("PROXY", "Stats: args %v err %v\n", args, err)
		return MkRerrorWC(err.Code())
	}
	rets.Stat = *st
	db.DPrintf("PROXY", "Stat: req %v rets %v\n", args, rets)
	return nil
}

func (npc *NpConn) Wstat(args *sp.Twstat, rets *sp.Rwstat) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Fid)
	if !ok {
		return MkRerrorWC(fcall.TErrNotfound)
	}
	err := npc.fidc.Wstat(fid, &args.Stat)
	if err != nil {
		db.DPrintf("PROXY", "Wstats: args %v err %v\n", args, err)
		return MkRerrorWC(err.Code())
	}
	db.DPrintf("PROXY", "Wstat: req %v rets %v\n", args, rets)
	return nil
}

func (npc *NpConn) Renameat(args *sp.Trenameat, rets *sp.Rrenameat) *sp.Rerror {
	return MkRerrorWC(fcall.TErrNotSupported)
}

func (npc *NpConn) ReadV(args *sp.TreadV, rets *sp.Rread) *sp.Rerror {
	return MkRerrorWC(fcall.TErrNotSupported)
}

func (npc *NpConn) WriteV(args *sp.TwriteV, rets *sp.Rwrite) *sp.Rerror {
	return MkRerrorWC(fcall.TErrNotSupported)
}

func (npc *NpConn) GetFile(args *sp.Tgetfile, rets *sp.Rgetfile) *sp.Rerror {
	return nil
}

func (npc *NpConn) SetFile(args *sp.Tsetfile, rets *sp.Rwrite) *sp.Rerror {
	return nil
}

func (npc *NpConn) PutFile(args *sp.Tputfile, rets *sp.Rwrite) *sp.Rerror {
	return nil
}

func (npc *NpConn) WriteRead(args *sp.Twriteread, rets *sp.Rwriteread) *sp.Rerror {
	return nil
}

func (npc *NpConn) Snapshot() []byte {
	return nil
}
