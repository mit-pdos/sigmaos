package proxy

import (
	"os/user"
	"sync"

	db "ulambda/debug"
	"ulambda/fidclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/pathclnt"
	"ulambda/protclnt"
	"ulambda/sessstatesrv"
	"ulambda/threadmgr"
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
	fcall := np.MakeFcall(reply, 0, nil, nil, np.NoFence)
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

// The connection from the kernel/client
type NpConn struct {
	mu    sync.Mutex
	clnt  *protclnt.Clnt
	uname string
	fidc  *fidclnt.FidClnt
	pc    *pathclnt.PathClnt
	named []string
}

func makeNpConn(named []string) *NpConn {
	npc := &NpConn{}
	npc.clnt = protclnt.MakeClnt()
	npc.named = named
	npc.fidc = fidclnt.MakeFidClnt()
	npc.pc = pathclnt.MakePathClnt(npc.fidc, np.Tsize(1_000_000))
	return npc
}

func (npc *NpConn) Version(args np.Tversion, rets *np.Rversion) *np.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (npc *NpConn) Auth(args np.Tauth, rets *np.Rauth) *np.Rerror {
	return np.MkRerrorWC(np.TErrNotSupported)
}

func (npc *NpConn) Attach(args np.Tattach, rets *np.Rattach) *np.Rerror {
	u, error := user.Current()
	if error != nil {
		return &np.Rerror{error.Error()}
	}
	npc.uname = u.Uid

	fid, err := npc.fidc.Attach(npc.uname, npc.named, "", "")
	if err != nil {
		return err.Rerror()
	}
	rets.Qid = npc.fidc.Qid(fid)
	return nil
}

func (npc *NpConn) Detach(rets *np.Rdetach) *np.Rerror {
	db.DPrintf("PROXY", "Detach\n")
	return nil
}

func (npc *NpConn) Walk(args np.Twalk, rets *np.Rwalk) *np.Rerror {
	fid, err := npc.pc.Walk(args.Wnames, true, nil)
	db.DPrintf("PROXY", "Walk %v args fid %v err %v\n", args, fid, err)
	if err != nil {
		return err.Rerror()
	}
	rets.Qids = npc.pc.Qids(fid)
	return nil
}

func (npc *NpConn) Open(args np.Topen, rets *np.Ropen) *np.Rerror {
	qid, err := npc.fidc.Open(args.Fid, args.Mode)
	if err != nil {
		return err.RerrorWC()
	}
	rets.Qid = qid
	return nil
}

func (npc *NpConn) Watch(args np.Twatch, rets *np.Ropen) *np.Rerror {
	return nil
}

func (npc *NpConn) Create(args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	fid, err := npc.fidc.Create(args.Fid, args.Name, args.Perm, args.Mode)
	if err != nil {
		return err.RerrorWC()
	}
	rets.Qid = npc.pc.Qid(fid)
	return nil
}

func (npc *NpConn) Clunk(args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	err := npc.fidc.Clunk(args.Fid)
	if err != nil {
		return err.RerrorWC()
	}
	return nil
}

func (npc *NpConn) Flush(args np.Tflush, rets *np.Rflush) *np.Rerror {
	return nil
}

func (npc *NpConn) Read(args np.Tread, rets *np.Rread) *np.Rerror {
	d, err := npc.fidc.ReadVU(args.Fid, args.Offset, args.Count, np.NoV)
	if err != nil {
		return err.RerrorWC()
	}
	rets.Data = d
	return nil
}

func (npc *NpConn) Write(args np.Twrite, rets *np.Rwrite) *np.Rerror {
	n, err := npc.fidc.WriteV(args.Fid, args.Offset, args.Data, np.NoV)
	if err != nil {
		return err.RerrorWC()
	}
	rets.Count = n
	return nil
}

func (npc *NpConn) Remove(args np.Tremove, rets *np.Rremove) *np.Rerror {
	err := npc.fidc.Remove(args.Fid)
	if err != nil {
		return err.RerrorWC()
	}
	return nil
}

func (npc *NpConn) RemoveFile(args np.Tremovefile, rets *np.Rremove) *np.Rerror {
	return nil
}

func (npc *NpConn) Stat(args np.Tstat, rets *np.Rstat) *np.Rerror {
	st, err := npc.fidc.Stat(args.Fid)
	if err != nil {
		return err.RerrorWC()
	}
	rets.Stat = *st
	db.DPrintf("PROXY", "Stat: req %v rets %v\n", args, rets)
	return nil
}

func (npc *NpConn) Wstat(args np.Twstat, rets *np.Rwstat) *np.Rerror {
	err := npc.fidc.Wstat(args.Fid, &args.Stat)
	if err != nil {
		return err.RerrorWC()
	}
	return nil
}

func (npc *NpConn) Renameat(args np.Trenameat, rets *np.Rrenameat) *np.Rerror {
	return np.MkRerrorWC(np.TErrNotSupported)
}

func (npc *NpConn) ReadV(args np.TreadV, rets *np.Rread) *np.Rerror {
	return np.MkRerrorWC(np.TErrNotSupported)
}

func (npc *NpConn) WriteV(args np.TwriteV, rets *np.Rwrite) *np.Rerror {
	return np.MkRerrorWC(np.TErrNotSupported)
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
