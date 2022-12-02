package protclnt

import (
	"sync/atomic"

	"sigmaos/fcall"
	"sigmaos/path"
	"sigmaos/rand"
	"sigmaos/sessclnt"
	np "sigmaos/sigmap"
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
	seqno np.Tseqno
	sm    *sessclnt.Mgr
}

func MakeClnt() *Clnt {
	clnt := &Clnt{}
	clnt.seqno = 0
	clnt.id = clid
	clnt.sm = sessclnt.MakeMgr(clnt.id)
	return clnt
}

func (clnt *Clnt) ReadSeqNo() np.Tseqno {
	return np.Tseqno(atomic.LoadUint64((*uint64)(&clnt.seqno)))
}

func (clnt *Clnt) CallServer(addrs []string, args np.Tmsg, fence *np.Tfence) (np.Tmsg, *fcall.Err) {
	reply, err := clnt.sm.RPC(addrs, args, fence)
	if err != nil {
		return nil, err
	}
	rmsg, ok := reply.(*np.Rerror)
	if ok {
		return nil, fcall.String2Err(rmsg.Ename)
	}
	return reply, nil
}

func (clnt *Clnt) Attach(addrs []string, uname string, fid np.Tfid, path path.Path) (*np.Rattach, *fcall.Err) {
	args := &np.Tattach{fid, np.NoFid, uname, path.String()}
	reply, err := clnt.CallServer(addrs, args, np.MakeFenceNull())
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rattach)
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

func (pclnt *ProtClnt) Call(args np.Tmsg, f *np.Tfence) (np.Tmsg, *fcall.Err) {
	return pclnt.clnt.CallServer(pclnt.addrs, args, f)
}

func (pclnt *ProtClnt) CallNoFence(args np.Tmsg) (np.Tmsg, *fcall.Err) {
	return pclnt.clnt.CallServer(pclnt.addrs, args, np.MakeFenceNull())
}

func (pclnt *ProtClnt) Flush(tag np.Ttag) *fcall.Err {
	args := &np.Tflush{tag}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	_, ok := reply.(*np.Rflush)
	if !ok {
		return fcall.MkErr(fcall.TErrBadFcall, "Rflush")
	}
	return nil
}

func (pclnt *ProtClnt) Walk(fid np.Tfid, nfid np.Tfid, path path.Path) (*np.Rwalk, *fcall.Err) {
	args := np.MkTwalk(fid, nfid, path)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rwalk)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rwalk")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Create(fid np.Tfid, name string, perm np.Tperm, mode np.Tmode) (*np.Rcreate, *fcall.Err) {
	args := &np.Tcreate{fid, name, perm, mode}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rcreate)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rcreate")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Remove(fid np.Tfid) *fcall.Err {
	args := &np.Tremove{fid}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	_, ok := reply.(*np.Rremove)
	if !ok {
		return fcall.MkErr(fcall.TErrBadFcall, "Rremove")
	}
	return nil
}

func (pclnt *ProtClnt) RemoveF(fid np.Tfid, f *np.Tfence) *fcall.Err {
	args := &np.Tremove{fid}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return err
	}
	_, ok := reply.(*np.Rremove)
	if !ok {
		return fcall.MkErr(fcall.TErrBadFcall, "Rremove")
	}
	return nil
}

func (pclnt *ProtClnt) RemoveFile(fid np.Tfid, wnames path.Path, resolve bool, f *np.Tfence) *fcall.Err {
	args := &np.Tremovefile{fid, wnames, resolve}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return err
	}
	_, ok := reply.(*np.Rremove)
	if !ok {
		return fcall.MkErr(fcall.TErrBadFcall, "Rremovefile")
	}
	return nil
}

func (pclnt *ProtClnt) Clunk(fid np.Tfid) *fcall.Err {
	args := &np.Tclunk{fid}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	_, ok := reply.(*np.Rclunk)
	if !ok {
		return fcall.MkErr(fcall.TErrBadFcall, "Rclunk")
	}
	return nil
}

func (pclnt *ProtClnt) Open(fid np.Tfid, mode np.Tmode) (*np.Ropen, *fcall.Err) {
	args := &np.Topen{fid, mode}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Ropen)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Ropen")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Watch(fid np.Tfid) *fcall.Err {
	args := &np.Twatch{fid}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	_, ok := reply.(*np.Ropen)
	if !ok {
		return fcall.MkErr(fcall.TErrBadFcall, "Rwatch")
	}
	return nil
}

func (pclnt *ProtClnt) Read(fid np.Tfid, offset np.Toffset, cnt np.Tsize) (*np.Rread, *fcall.Err) {
	args := &np.Tread{fid, offset, cnt}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rread)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rread")
	}
	return msg, nil
}

func (pclnt *ProtClnt) ReadVF(fid np.Tfid, offset np.Toffset, cnt np.Tsize, f *np.Tfence, v np.TQversion) (*np.Rread, *fcall.Err) {
	args := &np.TreadV{fid, offset, cnt, v}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rread)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rread")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Write(fid np.Tfid, offset np.Toffset, data []byte) (*np.Rwrite, *fcall.Err) {
	args := &np.Twrite{fid, offset, data}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rwrite)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rwrite")
	}
	return msg, nil
}

func (pclnt *ProtClnt) WriteVF(fid np.Tfid, offset np.Toffset, f *np.Tfence, v np.TQversion, data []byte) (*np.Rwrite, *fcall.Err) {
	args := &np.TwriteV{fid, offset, v, data}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rwrite)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rwrite")
	}
	return msg, nil
}

func (pclnt *ProtClnt) WriteRead(fid np.Tfid, data []byte) (*np.Rwriteread, *fcall.Err) {
	args := &np.Twriteread{}
	args.Fid = uint64(fid)
	args.Data = data
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rwriteread)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rwriteread")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Stat(fid np.Tfid) (*np.Rstat, *fcall.Err) {
	args := np.MkTstat(fid)
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rstat)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rstat")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Wstat(fid np.Tfid, st *np.Stat) (*np.Rwstat, *fcall.Err) {
	args := &np.Twstat{fid, 0, *st}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rwstat)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rwstat")
	}
	return msg, nil
}

func (pclnt *ProtClnt) WstatF(fid np.Tfid, st *np.Stat, f *np.Tfence) (*np.Rwstat, *fcall.Err) {
	args := &np.Twstat{fid, 0, *st}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rwstat)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rwstat")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Renameat(oldfid np.Tfid, oldname string, newfid np.Tfid, newname string, f *np.Tfence) (*np.Rrenameat, *fcall.Err) {
	args := &np.Trenameat{oldfid, oldname, newfid, newname}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rrenameat)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rrenameat")
	}
	return msg, nil
}

func (pclnt *ProtClnt) GetFile(fid np.Tfid, path path.Path, mode np.Tmode, offset np.Toffset, cnt np.Tsize, resolve bool, f *np.Tfence) (*np.Rgetfile, *fcall.Err) {
	args := &np.Tgetfile{fid, mode, offset, cnt, path, resolve}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rgetfile)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rgetfile")
	}
	return msg, nil
}

func (pclnt *ProtClnt) SetFile(fid np.Tfid, path path.Path, mode np.Tmode, offset np.Toffset, resolve bool, f *np.Tfence, data []byte) (*np.Rwrite, *fcall.Err) {
	args := &np.Tsetfile{fid, mode, offset, path, resolve, data}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rwrite)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rwrite")
	}
	return msg, nil
}

func (pclnt *ProtClnt) PutFile(fid np.Tfid, path path.Path, mode np.Tmode, perm np.Tperm, offset np.Toffset, f *np.Tfence, data []byte) (*np.Rwrite, *fcall.Err) {
	args := &np.Tputfile{fid, mode, perm, offset, path, data}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rwrite)
	if !ok {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "Rwrite")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Detach() *fcall.Err {
	args := &np.Tdetach{0, 0}
	_, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	return nil
}
