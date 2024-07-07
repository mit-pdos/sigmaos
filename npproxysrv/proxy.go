// Package npproxy (9P proxy) provides access to sigmaos from the
// linux command line and by mounting root realm's named at /mnt/9p as
// an 9P file system (see mount.sh), and translating 9P into sigmaP
// (and back).
package npproxysrv

import (
	"net"
	"sync"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/fidclnt"
	"sigmaos/netproxyclnt"
	"sigmaos/npproxysrv/npcodec"
	"sigmaos/path"
	"sigmaos/pathclnt"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/sigmaprotsrv"
)

type proxyConn struct {
	p    *sp.Tprincipal
	conn net.Conn
	sess *NpSess
}

func (pc *proxyConn) ReportError(err error) {
	db.DPrintf(db.NPPROXY, "ReportError %v err %v\n", pc, err)
}

func (pc *proxyConn) ServeRequest(fc demux.CallI) (demux.CallI, *serr.Err) {
	fm := fc.(*sessp.FcallMsg)
	db.DPrintf(db.NPPROXY, "ServeRequest %v\n", fm)
	msg, iov, rerror, _, _ := sigmaprotsrv.Dispatch(pc.sess, fm.Msg, fm.Iov)
	if rerror != nil {
		msg = rerror
	}
	reply := sessp.NewFcallMsg(msg, nil, sessp.Tsession(fm.Fc.Session), nil)
	reply.Iov = iov
	reply.Fc.Seqno = fm.Fc.Seqno
	return reply, nil
}

type Npd struct {
	lip sp.Tip
	pe  *proc.ProcEnv
	npc *netproxyclnt.NetProxyClnt
}

func NewNpd(pe *proc.ProcEnv, npc *netproxyclnt.NetProxyClnt, lip sp.Tip) *Npd {
	return &Npd{
		lip: lip,
		pe:  pe,
		npc: npc,
	}
}

// Create a sigmap session for conn
func (npd *Npd) NewConn(p *sp.Tprincipal, conn net.Conn) *demux.DemuxSrv {
	sess := newNpSess(npd.pe, npd.npc, string(npd.lip))
	pc := &proxyConn{
		p:    p,
		conn: conn,
		sess: sess,
	}
	return demux.NewDemuxSrv(pc, npcodec.NewTransport(conn))
}

// The protsrv session from the kernel/client
type NpSess struct {
	mu        sync.Mutex
	principal *sp.Tprincipal
	fidc      *fidclnt.FidClnt
	pc        *pathclnt.PathClnt
	pe        *proc.ProcEnv
	fm        *fidMap
	qm        *qidMap
	cid       sp.TclntId
}

func newNpSess(pe *proc.ProcEnv, npcs *netproxyclnt.NetProxyClnt, lip string) *NpSess {
	npc := &NpSess{}
	npc.pe = pe
	npc.fidc = fidclnt.NewFidClnt(pe, npcs)
	npc.principal = pe.GetPrincipal()
	npc.pc = pathclnt.NewPathClnt(pe, npc.fidc)
	npc.fm = newFidMap()
	npc.qm = newQidMap()
	npc.cid = sp.TclntId(rand.Uint64())
	return npc
}

func (npc *NpSess) Version(args *sp.Tversion, rets *sp.Rversion) *sp.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (npc *NpSess) Auth(args *sp.Tauth, rets *sp.Rauth) *sp.Rerror {
	return sp.NewRerrorCode(serr.TErrNotSupported)
}

func (npc *NpSess) Attach(args *sp.Tattach, rets *sp.Rattach) (sp.TclntId, *sp.Rerror) {
	ep, error := npc.pc.MntClnt().GetNamedEndpointRealm(sp.ROOTREALM)
	if error != nil {
		db.DPrintf(db.ERROR, "Error GetNamedEndpoint: %v", error)
		return sp.NoClntId, sp.NewRerrorSerr(serr.NewErrError(error))
	}
	fid, err := npc.fidc.Attach(npc.pe.GetSecrets(), npc.cid, ep, "", "")
	if err != nil {
		db.DPrintf(db.NPPROXY, "Attach args %v err %v\n", args, err)
		return sp.NoClntId, sp.NewRerrorSerr(err)
	}
	if err := npc.pc.MntClnt().Mount(fid, sp.NAMED); err != nil {
		db.DPrintf(db.NPPROXY, "Attach args %v mount err %v\n", args, err)
		return sp.NoClntId, sp.NewRerrorSerr(serr.NewErrError(err))
	}
	rets.Qid = npc.qm.Insert(path.Tpathname{sp.NAMED}, []*sp.Tqid{npc.fidc.Qid(fid)})[0]
	npc.fm.mapTo(args.Tfid(), fid)
	npc.fidc.Lookup(fid).SetPath(path.Split(sp.NAMED))
	db.DPrintf(db.NPPROXY, "Attach args %v rets %v fid %v\n", args, rets, fid)
	return args.TclntId(), nil
}

func (npc *NpSess) Detach(args *sp.Tdetach, rets *sp.Rdetach) *sp.Rerror {
	db.DPrintf(db.NPPROXY, "Detach\n")
	return nil
}

func (npc *NpSess) Walk(args *sp.Twalk, rets *sp.Rwalk) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.NewRerrorCode(serr.TErrNotfound)
	}
	fid1, err := npc.pc.Walk(fid, args.Wnames, sp.NewPrincipal(
		sp.TprincipalID("proxy"),
		sp.ROOTREALM,
	))
	if err != nil {
		db.DPrintf(db.NPPROXY, "Walk args %v err: %v\n", args, err)
		return sp.NewRerrorSerr(err)
	}

	ch := npc.fidc.Lookup(fid1)
	qids := ch.Qids()
	qids = qids[len(qids)-len(args.Wnames):]

	rets.Qids = npc.qm.Insert(ch.Path(), qids)
	npc.fm.mapTo(args.Tnewfid(), fid1)
	return nil
}

func (npc *NpSess) Open(args *sp.Topen, rets *sp.Ropen) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.NewRerrorCode(serr.TErrNotfound)
	}
	qid, err := npc.fidc.Open(fid, args.Tmode())
	if err != nil {
		db.DPrintf(db.NPPROXY, "Open args %v err: %v\n", args, err)
		return sp.NewRerrorSerr(err)
	}
	rets.Qid = qid.Proto()
	db.DPrintf(db.NPPROXY, "Open args %v rets: %v\n", args, rets)
	return nil
}

func (npc *NpSess) Watch(args *sp.Twatch, rets *sp.Ropen) *sp.Rerror {
	return nil
}

func (npc *NpSess) Create(args *sp.Tcreate, rets *sp.Rcreate) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.NewRerrorCode(serr.TErrNotfound)
	}
	fid1, err := npc.fidc.Create(fid, args.Name, args.Tperm(), args.Tmode(), sp.NoLeaseId, sp.NoFence())
	if err != nil {
		db.DPrintf(db.NPPROXY, "Create args %v err: %v\n", args, err)
		return sp.NewRerrorSerr(err)
	}
	if fid != fid1 {
		db.DPrintf(db.ALWAYS, "Create fid %v fid1 %v\n", fid, fid1)
	}
	rets.Qid = npc.pc.Qid(fid1).Proto()
	db.DPrintf(db.NPPROXY, "Create args %v rets: %v\n", args, rets)
	return nil
}

func (npc *NpSess) Clunk(args *sp.Tclunk, rets *sp.Rclunk) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.NewRerrorCode(serr.TErrNotfound)
	}
	ch := npc.fidc.Lookup(fid)
	npc.qm.Clunk(ch.Path(), ch.Lastqid())
	err := npc.fidc.Clunk(fid)
	if err != nil {
		db.DPrintf(db.NPPROXY, "Clunk: args %v err %v\n", args, err)
		return sp.NewRerrorSerr(err)
	}
	npc.fm.delete(args.Tfid())
	db.DPrintf(db.NPPROXY, "Clunk: args %v rets %v\n", args, rets)
	return nil
}

func (npc *NpSess) Remove(args *sp.Tremove, rets *sp.Rremove) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.NewRerrorCode(serr.TErrNotfound)
	}
	err := npc.fidc.Remove(fid, sp.NullFence())
	if err != nil {
		db.DPrintf(db.NPPROXY, "Remove: args %v err %v\n", args, err)
		return sp.NewRerrorSerr(err)
	}
	npc.fm.delete(args.Tfid())
	db.DPrintf(db.NPPROXY, "Remove: args %v rets %v\n", args, rets)
	return nil
}

func (npc *NpSess) RemoveFile(args *sp.Tremovefile, rets *sp.Rremove) *sp.Rerror {
	return nil
}

func (npc *NpSess) Stat(args *sp.Trstat, rets *sp.Rrstat) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.NewRerrorCode(serr.TErrNotfound)
	}
	st, err := npc.fidc.Stat(fid)
	if err != nil {
		db.DPrintf(db.NPPROXY, "Stats: args %v err %v\n", args, err)
		return sp.NewRerrorSerr(err)
	}
	rets.Stat = st.StatProto()
	db.DPrintf(db.NPPROXY, "Stat: req %v rets %v\n", args, rets)
	return nil
}

func (npc *NpSess) Wstat(args *sp.Twstat, rets *sp.Rwstat) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.NewRerrorCode(serr.TErrNotfound)
	}
	err := npc.fidc.Wstat(fid, sp.NewStatProto(args.Stat), sp.NullFence())
	if err != nil {
		db.DPrintf(db.NPPROXY, "Wstats: args %v err %v\n", args, err)
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.NPPROXY, "Wstat: req %v rets %v\n", args, rets)
	return nil
}

func (npc *NpSess) Renameat(args *sp.Trenameat, rets *sp.Rrenameat) *sp.Rerror {
	return sp.NewRerrorCode(serr.TErrNotSupported)
}

func (npc *NpSess) ReadF(args *sp.TreadF, rets *sp.Rread) ([]byte, *sp.Rerror) {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return nil, sp.NewRerrorCode(serr.TErrNotfound)
	}
	b := make([]byte, args.Tcount())
	cnt, err := npc.fidc.ReadF(fid, args.Toffset(), b, sp.NullFence())
	if err != nil {
		db.DPrintf(db.NPPROXY, "Read: args %v err %v\n", args, err)
		return nil, sp.NewRerrorSerr(err)
	}
	b = b[:cnt]
	db.DPrintf(db.NPPROXY, "ReadUV: args %v rets %v %d", args, rets, cnt)
	qid := npc.pc.Qid(fid)
	if sp.Qtype(qid.Type)&sp.QTDIR == sp.QTDIR {
		d1, err1 := Sp2NpDir(b, args.Tcount())
		if err != nil {
			db.DPrintf(db.NPPROXY, "Read: Sp2NpDir err %v\n", err1)
			return nil, sp.NewRerrorSerr(serr.NewErrError(err1))
		}
		b = d1
	}
	rets.Count = uint32(len(b))
	db.DPrintf(db.NPPROXY, "Read: args %v rets %v %v\n", args, rets, cnt)
	return b, nil
}

func (npc *NpSess) WriteF(args *sp.TwriteF, data []byte, rets *sp.Rwrite) *sp.Rerror {
	fid, ok := npc.fm.lookup(args.Tfid())
	if !ok {
		return sp.NewRerrorCode(serr.TErrNotfound)
	}
	n, err := npc.fidc.WriteF(fid, args.Toffset(), data, sp.NullFence())
	if err != nil {
		db.DPrintf(db.NPPROXY, "Write: args %v err %v\n", args, err)
		return sp.NewRerrorSerr(err)
	}
	rets.Count = uint32(n)
	db.DPrintf(db.NPPROXY, "Write: args %v rets %v\n", args, rets)
	return nil
}

func (npc *NpSess) GetFile(args *sp.Tgetfile, rets *sp.Rread) ([]byte, *sp.Rerror) {
	return nil, nil
}

func (npc *NpSess) PutFile(args *sp.Tputfile, d []byte, rets *sp.Rwrite) *sp.Rerror {
	return nil
}

func (npc *NpSess) WriteRead(args *sp.Twriteread, iov sessp.IoVec, rets *sp.Rread) (sessp.IoVec, *sp.Rerror) {
	return nil, nil
}
