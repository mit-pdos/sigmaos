package protclnt

import (
	"sync/atomic"

	"sigmaos/path"
	"sigmaos/rand"
	"sigmaos/serr"
	"sigmaos/sessclnt"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

// Each proc has a unique client ID.
var clid sessp.Tclient = 0

func init() {
	if clid == 0 {
		clid = sessp.Tclient(rand.Uint64())
	}
}

type Clnt struct {
	id    sessp.Tclient
	seqno sessp.Tseqno
	sm    *sessclnt.Mgr
	realm sp.Trealm
}

func MakeClnt(clntnet string) *Clnt {
	clnt := &Clnt{}
	clnt.seqno = 0
	clnt.id = clid
	clnt.sm = sessclnt.MakeMgr(clnt.id, clntnet)
	return clnt
}

func (clnt *Clnt) ReadSeqNo() sessp.Tseqno {
	return sessp.Tseqno(atomic.LoadUint64((*uint64)(&clnt.seqno)))
}

func (clnt *Clnt) CallServer(addrs sp.Taddrs, args sessp.Tmsg, data []byte, fence *sessp.Tfence) (*sessp.FcallMsg, *serr.Err) {
	reply, err := clnt.sm.RPC(addrs, args, data, fence)
	if err != nil {
		return nil, err
	}
	rmsg, ok := reply.Msg.(*sp.Rerror)
	if ok {
		return nil, sp.MkErr(rmsg)
	}
	return reply, nil
}

func (clnt *Clnt) Attach(addrs sp.Taddrs, uname sp.Tuname, cid sp.TclntId, fid sp.Tfid, path path.Path) (*sp.Rattach, *serr.Err) {
	args := sp.MkTattach(fid, sp.NoFid, uname, cid, path)
	reply, err := clnt.CallServer(addrs, args, nil, sessp.MakeFenceNull())
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rattach)
	if !ok {
		return nil, serr.MkErr(serr.TErrBadFcall, "clnt")
	}
	return msg, nil
}

func (clnt *Clnt) Exit(cid sp.TclntId) *serr.Err {
	scs := clnt.sm.SessClnts()
	for _, sc := range scs {
		sc.Detach(cid)
	}
	return nil
}

func (clnt *Clnt) MakeProtClnt(addrs sp.Taddrs) *ProtClnt {
	protclnt := &ProtClnt{addrs, clnt}
	return protclnt
}

type ProtClnt struct {
	addrs sp.Taddrs
	clnt  *Clnt
}

func (pclnt *ProtClnt) Servers() sp.Taddrs {
	return pclnt.addrs
}

func (pclnt *ProtClnt) Disconnect() *serr.Err {
	return pclnt.clnt.sm.Disconnect(pclnt.addrs)
}

func (pclnt *ProtClnt) Call(args sessp.Tmsg, f *sessp.Tfence) (*sessp.FcallMsg, *serr.Err) {
	return pclnt.clnt.CallServer(pclnt.addrs, args, nil, f)
}

func (pclnt *ProtClnt) CallData(args sessp.Tmsg, data []byte, f *sessp.Tfence) (*sessp.FcallMsg, *serr.Err) {
	return pclnt.clnt.CallServer(pclnt.addrs, args, data, f)
}

func (pclnt *ProtClnt) CallNoFence(args sessp.Tmsg) (*sessp.FcallMsg, *serr.Err) {
	return pclnt.clnt.CallServer(pclnt.addrs, args, nil, sessp.MakeFenceNull())
}

func (pclnt *ProtClnt) CallNoFenceData(args sessp.Tmsg, data []byte) (*sessp.FcallMsg, *serr.Err) {
	return pclnt.clnt.CallServer(pclnt.addrs, args, data, sessp.MakeFenceNull())
}

func (pclnt *ProtClnt) Walk(fid sp.Tfid, nfid sp.Tfid, path path.Path) (*sp.Rwalk, *serr.Err) {
	args := sp.MkTwalk(fid, nfid, path)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rwalk)
	if !ok {
		return nil, serr.MkErr(serr.TErrBadFcall, "Rwalk")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Create(fid sp.Tfid, name string, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId) (*sp.Rcreate, *serr.Err) {
	args := sp.MkTcreate(fid, name, perm, mode, lid)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rcreate)
	if !ok {
		return nil, serr.MkErr(serr.TErrBadFcall, "Rcreate")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Remove(fid sp.Tfid) *serr.Err {
	args := sp.MkTremove(fid)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	_, ok := reply.Msg.(*sp.Rremove)
	if !ok {
		return serr.MkErr(serr.TErrBadFcall, "Rremove")
	}
	return nil
}

func (pclnt *ProtClnt) RemoveF(fid sp.Tfid, f *sessp.Tfence) *serr.Err {
	args := sp.MkTremove(fid)
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return err
	}
	_, ok := reply.Msg.(*sp.Rremove)
	if !ok {
		return serr.MkErr(serr.TErrBadFcall, "Rremove")
	}
	return nil
}

func (pclnt *ProtClnt) RemoveFile(fid sp.Tfid, wnames path.Path, resolve bool, f *sessp.Tfence) *serr.Err {
	args := sp.MkTremovefile(fid, wnames, resolve)
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return err
	}
	_, ok := reply.Msg.(*sp.Rremove)
	if !ok {
		return serr.MkErr(serr.TErrBadFcall, "Rremovefile")
	}
	return nil
}

func (pclnt *ProtClnt) Clunk(fid sp.Tfid) *serr.Err {
	args := sp.MkTclunk(fid)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	_, ok := reply.Msg.(*sp.Rclunk)
	if !ok {
		return serr.MkErr(serr.TErrBadFcall, "Rclunk")
	}
	return nil
}

func (pclnt *ProtClnt) Open(fid sp.Tfid, mode sp.Tmode) (*sp.Ropen, *serr.Err) {
	args := sp.MkTopen(fid, mode)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Ropen)
	if !ok {
		return nil, serr.MkErr(serr.TErrBadFcall, "Ropen")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Watch(fid sp.Tfid) *serr.Err {
	args := sp.MkTwatch(fid)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	_, ok := reply.Msg.(*sp.Ropen)
	if !ok {
		return serr.MkErr(serr.TErrBadFcall, "Rwatch")
	}
	return nil
}

func (pclnt *ProtClnt) ReadVF(fid sp.Tfid, offset sp.Toffset, cnt sessp.Tsize, f *sessp.Tfence, v sp.TQversion) ([]byte, *serr.Err) {
	args := sp.MkReadV(fid, offset, cnt, v)
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	_, ok := reply.Msg.(*sp.Rread)
	if !ok {
		return nil, serr.MkErr(serr.TErrBadFcall, "Rread")
	}
	return reply.Data, nil
}

func (pclnt *ProtClnt) WriteVF(fid sp.Tfid, offset sp.Toffset, f *sessp.Tfence, v sp.TQversion, data []byte) (*sp.Rwrite, *serr.Err) {
	args := sp.MkTwriteV(fid, offset, v)
	reply, err := pclnt.CallData(args, data, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rwrite)
	if !ok {
		return nil, serr.MkErr(serr.TErrBadFcall, "Rwrite")
	}
	return msg, nil
}

func (pclnt *ProtClnt) WriteRead(fid sp.Tfid, data []byte) ([]byte, *serr.Err) {
	args := sp.MkTwriteread(fid)
	reply, err := pclnt.CallNoFenceData(args, data)
	if err != nil {
		return nil, err
	}
	_, ok := reply.Msg.(*sp.Rread)
	if !ok {
		return nil, serr.MkErr(serr.TErrBadFcall, "Rwriteread")
	}
	return reply.Data, nil
}

func (pclnt *ProtClnt) Stat(fid sp.Tfid) (*sp.Rstat, *serr.Err) {
	args := sp.MkTstat(fid)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rstat)
	if !ok {
		return nil, serr.MkErr(serr.TErrBadFcall, "Rstat")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Wstat(fid sp.Tfid, st *sp.Stat) (*sp.Rwstat, *serr.Err) {
	args := sp.MkTwstat(fid, st)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rwstat)
	if !ok {
		return nil, serr.MkErr(serr.TErrBadFcall, "Rwstat")
	}
	return msg, nil
}

func (pclnt *ProtClnt) WstatF(fid sp.Tfid, st *sp.Stat, f *sessp.Tfence) (*sp.Rwstat, *serr.Err) {
	args := sp.MkTwstat(fid, st)
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rwstat)
	if !ok {
		return nil, serr.MkErr(serr.TErrBadFcall, "Rwstat")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Renameat(oldfid sp.Tfid, oldname string, newfid sp.Tfid, newname string, f *sessp.Tfence) (*sp.Rrenameat, *serr.Err) {
	args := sp.MkTrenameat(oldfid, oldname, newfid, newname)
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rrenameat)
	if !ok {
		return nil, serr.MkErr(serr.TErrBadFcall, "Rrenameat")
	}
	return msg, nil
}

func (pclnt *ProtClnt) GetFile(fid sp.Tfid, path path.Path, mode sp.Tmode, offset sp.Toffset, cnt sessp.Tsize, resolve bool, f *sessp.Tfence) ([]byte, *serr.Err) {
	args := sp.MkTgetfile(fid, mode, offset, cnt, path, resolve)
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	_, ok := reply.Msg.(*sp.Rread)
	if !ok {
		return nil, serr.MkErr(serr.TErrBadFcall, "Rgetfile")
	}
	return reply.Data, nil
}

func (pclnt *ProtClnt) PutFile(fid sp.Tfid, path path.Path, mode sp.Tmode, perm sp.Tperm, offset sp.Toffset, resolve bool, f *sessp.Tfence, data []byte, lid sp.TleaseId) (*sp.Rwrite, *serr.Err) {
	args := sp.MkTputfile(fid, mode, perm, offset, path, resolve, lid)
	reply, err := pclnt.CallData(args, data, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rwrite)
	if !ok {
		return nil, serr.MkErr(serr.TErrBadFcall, "Rwrite")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Detach(cid sp.TclntId) *serr.Err {
	args := sp.MkTdetach(0, 0, cid)
	_, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	return nil
}
