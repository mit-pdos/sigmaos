package protclnt

import (
	"sync/atomic"

	"sigmaos/fcall"
	"sigmaos/path"
	"sigmaos/rand"
	"sigmaos/sessclnt"
	sp "sigmaos/sigmap"
)

// Each proc has a unique client ID.
var clid fcall.Tclient = 0

func init() {
	if clid == 0 {
		clid = fcall.Tclient(rand.Uint64())
	}
}

type Clnt struct {
	id    fcall.Tclient
	seqno sp.Tseqno
	sm    *sessclnt.Mgr
}

func MakeClnt() *Clnt {
	clnt := &Clnt{}
	clnt.seqno = 0
	clnt.id = clid
	clnt.sm = sessclnt.MakeMgr(clnt.id)
	return clnt
}

func (clnt *Clnt) ReadSeqNo() sp.Tseqno {
	return sp.Tseqno(atomic.LoadUint64((*uint64)(&clnt.seqno)))
}

func (clnt *Clnt) CallServer(addrs []string, args fcall.Tmsg, data []byte, fence *sp.Tfence) (*sp.FcallMsg, *fcall.Err) {
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

func (clnt *Clnt) Attach(addrs []string, uname string, fid sp.Tfid, path path.Path) (*sp.Rattach, *fcall.Err) {
	args := sp.MkTattach(fid, sp.NoFid, uname, path)
	reply, err := clnt.CallServer(addrs, args, nil, sp.MakeFenceNull())
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rattach)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "clnt")
	}
	return msg, nil
}

func (clnt *Clnt) Exit() *fcall.Err {
	scs := clnt.sm.SessClnts()
	for _, sc := range scs {
		sc.Detach()
	}
	return nil
}

func (clnt *Clnt) MakeProtClnt(addrs []string) *ProtClnt {
	protclnt := &ProtClnt{addrs, clnt}
	return protclnt
}

type ProtClnt struct {
	addrs []string
	clnt  *Clnt
}

func (pclnt *ProtClnt) Servers() []string {
	return pclnt.addrs
}

func (pclnt *ProtClnt) Disconnect() *fcall.Err {
	return pclnt.clnt.sm.Disconnect(pclnt.addrs)
}

func (pclnt *ProtClnt) Call(args fcall.Tmsg, f *sp.Tfence) (*sp.FcallMsg, *fcall.Err) {
	return pclnt.clnt.CallServer(pclnt.addrs, args, nil, f)
}

func (pclnt *ProtClnt) CallData(args fcall.Tmsg, data []byte, f *sp.Tfence) (*sp.FcallMsg, *fcall.Err) {
	return pclnt.clnt.CallServer(pclnt.addrs, args, data, f)
}

func (pclnt *ProtClnt) CallNoFence(args fcall.Tmsg) (*sp.FcallMsg, *fcall.Err) {
	return pclnt.clnt.CallServer(pclnt.addrs, args, nil, sp.MakeFenceNull())
}

func (pclnt *ProtClnt) CallNoFenceData(args fcall.Tmsg, data []byte) (*sp.FcallMsg, *fcall.Err) {
	return pclnt.clnt.CallServer(pclnt.addrs, args, data, sp.MakeFenceNull())
}

func (pclnt *ProtClnt) Walk(fid sp.Tfid, nfid sp.Tfid, path path.Path) (*sp.Rwalk, *fcall.Err) {
	args := sp.MkTwalk(fid, nfid, path)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rwalk)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rwalk")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Create(fid sp.Tfid, name string, perm sp.Tperm, mode sp.Tmode) (*sp.Rcreate, *fcall.Err) {
	args := sp.MkTcreate(fid, name, perm, mode)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rcreate)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rcreate")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Remove(fid sp.Tfid) *fcall.Err {
	args := sp.MkTremove(fid)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	_, ok := reply.Msg.(*sp.Rremove)
	if !ok {
		return fcall.MkErr(fcall.TErrBadFcall, "Rremove")
	}
	return nil
}

func (pclnt *ProtClnt) RemoveF(fid sp.Tfid, f *sp.Tfence) *fcall.Err {
	args := sp.MkTremove(fid)
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return err
	}
	_, ok := reply.Msg.(*sp.Rremove)
	if !ok {
		return fcall.MkErr(fcall.TErrBadFcall, "Rremove")
	}
	return nil
}

func (pclnt *ProtClnt) RemoveFile(fid sp.Tfid, wnames path.Path, resolve bool, f *sp.Tfence) *fcall.Err {
	args := sp.MkTremovefile(fid, wnames, resolve)
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return err
	}
	_, ok := reply.Msg.(*sp.Rremove)
	if !ok {
		return fcall.MkErr(fcall.TErrBadFcall, "Rremovefile")
	}
	return nil
}

func (pclnt *ProtClnt) Clunk(fid sp.Tfid) *fcall.Err {
	args := sp.MkTclunk(fid)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	_, ok := reply.Msg.(*sp.Rclunk)
	if !ok {
		return fcall.MkErr(fcall.TErrBadFcall, "Rclunk")
	}
	return nil
}

func (pclnt *ProtClnt) Open(fid sp.Tfid, mode sp.Tmode) (*sp.Ropen, *fcall.Err) {
	args := sp.MkTopen(fid, mode)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Ropen)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Ropen")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Watch(fid sp.Tfid) *fcall.Err {
	args := sp.MkTwatch(fid)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	_, ok := reply.Msg.(*sp.Ropen)
	if !ok {
		return fcall.MkErr(fcall.TErrBadFcall, "Rwatch")
	}
	return nil
}

func (pclnt *ProtClnt) ReadVF(fid sp.Tfid, offset sp.Toffset, cnt sp.Tsize, f *sp.Tfence, v sp.TQversion) ([]byte, *fcall.Err) {
	args := sp.MkReadV(fid, offset, cnt, v)
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	_, ok := reply.Msg.(*sp.Rread)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rread")
	}
	return reply.Data, nil
}

func (pclnt *ProtClnt) WriteVF(fid sp.Tfid, offset sp.Toffset, f *sp.Tfence, v sp.TQversion, data []byte) (*sp.Rwrite, *fcall.Err) {
	args := sp.MkTwriteV(fid, offset, v)
	reply, err := pclnt.CallData(args, data, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rwrite)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rwrite")
	}
	return msg, nil
}

func (pclnt *ProtClnt) WriteRead(fid sp.Tfid, data []byte) ([]byte, *fcall.Err) {
	args := sp.MkTwriteread(fid)
	reply, err := pclnt.CallNoFenceData(args, data)
	if err != nil {
		return nil, err
	}
	_, ok := reply.Msg.(*sp.Rread)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rwriteread")
	}
	return reply.Data, nil
}

func (pclnt *ProtClnt) Stat(fid sp.Tfid) (*sp.Rstat, *fcall.Err) {
	args := sp.MkTstat(fid)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rstat)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rstat")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Wstat(fid sp.Tfid, st *sp.Stat) (*sp.Rwstat, *fcall.Err) {
	args := sp.MkTwstat(fid, st)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rwstat)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rwstat")
	}
	return msg, nil
}

func (pclnt *ProtClnt) WstatF(fid sp.Tfid, st *sp.Stat, f *sp.Tfence) (*sp.Rwstat, *fcall.Err) {
	args := sp.MkTwstat(fid, st)
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rwstat)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rwstat")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Renameat(oldfid sp.Tfid, oldname string, newfid sp.Tfid, newname string, f *sp.Tfence) (*sp.Rrenameat, *fcall.Err) {
	args := sp.MkTrenameat(oldfid, oldname, newfid, newname)
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rrenameat)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rrenameat")
	}
	return msg, nil
}

func (pclnt *ProtClnt) GetFile(fid sp.Tfid, path path.Path, mode sp.Tmode, offset sp.Toffset, cnt sp.Tsize, resolve bool, f *sp.Tfence) ([]byte, *fcall.Err) {
	args := sp.MkTgetfile(fid, mode, offset, cnt, path, resolve)
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	_, ok := reply.Msg.(*sp.Rread)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rgetfile")
	}
	return reply.Data, nil
}

func (pclnt *ProtClnt) SetFile(fid sp.Tfid, path path.Path, mode sp.Tmode, offset sp.Toffset, resolve bool, f *sp.Tfence, data []byte) (*sp.Rwrite, *fcall.Err) {
	args := sp.MkTsetfile(fid, mode, offset, path, resolve)
	reply, err := pclnt.CallData(args, data, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rwrite)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rwrite")
	}
	return msg, nil
}

func (pclnt *ProtClnt) PutFile(fid sp.Tfid, path path.Path, mode sp.Tmode, perm sp.Tperm, offset sp.Toffset, f *sp.Tfence, data []byte) (*sp.Rwrite, *fcall.Err) {
	args := sp.MkTputfile(fid, mode, perm, offset, path)
	reply, err := pclnt.CallData(args, data, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rwrite)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rwrite")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Detach() *fcall.Err {
	args := sp.MkTdetach(0, 0)
	_, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	return nil
}
