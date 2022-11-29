package protclnt

import (
	"sync/atomic"

	"sigmaos/rand"
	"sigmaos/sessclnt"
	np "sigmaos/sigmap"
)

// Each proc has a unique client ID.
var clid np.Tclient = 0

func init() {
	if clid == 0 {
		clid = np.Tclient(rand.Uint64())
	}
}

type Clnt struct {
	id    np.Tclient
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

func (clnt *Clnt) CallServer(addrs []string, args np.Tmsg, fence *np.Tfence) (np.Tmsg, *np.Err) {
	reply, err := clnt.sm.RPC(addrs, args, fence)
	if err != nil {
		return nil, err
	}
	rmsg, ok := reply.(*np.Rerror)
	if ok {
		return nil, np.String2Err(rmsg.Ename)
	}
	return reply, nil
}

func (clnt *Clnt) Attach(addrs []string, uname string, fid np.Tfid, path np.Path) (*np.Rattach, *np.Err) {
	args := &np.Tattach{fid, np.NoFid, uname, path.String()}
	reply, err := clnt.CallServer(addrs, args, np.MakeFenceNull())
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rattach)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "clnt")
	}
	return msg, nil
}

func (clnt *Clnt) Exit() *np.Err {
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

func (pclnt *ProtClnt) Disconnect() *np.Err {
	return pclnt.clnt.sm.Disconnect(pclnt.addrs)
}

func (pclnt *ProtClnt) Call(args np.Tmsg, f *np.Tfence) (np.Tmsg, *np.Err) {
	return pclnt.clnt.CallServer(pclnt.addrs, args, f)
}

func (pclnt *ProtClnt) CallNoFence(args np.Tmsg) (np.Tmsg, *np.Err) {
	return pclnt.clnt.CallServer(pclnt.addrs, args, np.MakeFenceNull())
}

func (pclnt *ProtClnt) Flush(tag np.Ttag) *np.Err {
	args := &np.Tflush{tag}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	_, ok := reply.(*np.Rflush)
	if !ok {
		return np.MkErr(np.TErrBadFcall, "Rflush")
	}
	return nil
}

func (pclnt *ProtClnt) Walk(fid np.Tfid, nfid np.Tfid, path np.Path) (*np.Rwalk, *np.Err) {
	args := &np.Twalk{fid, nfid, path}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rwalk)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rwalk")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Create(fid np.Tfid, name string, perm np.Tperm, mode np.Tmode) (*np.Rcreate, *np.Err) {
	args := &np.Tcreate{fid, name, perm, mode}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rcreate)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rcreate")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Remove(fid np.Tfid) *np.Err {
	args := &np.Tremove{fid}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	_, ok := reply.(*np.Rremove)
	if !ok {
		return np.MkErr(np.TErrBadFcall, "Rremove")
	}
	return nil
}

func (pclnt *ProtClnt) RemoveF(fid np.Tfid, f *np.Tfence) *np.Err {
	args := &np.Tremove{fid}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return err
	}
	_, ok := reply.(*np.Rremove)
	if !ok {
		return np.MkErr(np.TErrBadFcall, "Rremove")
	}
	return nil
}

func (pclnt *ProtClnt) RemoveFile(fid np.Tfid, wnames np.Path, resolve bool, f *np.Tfence) *np.Err {
	args := &np.Tremovefile{fid, wnames, resolve}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return err
	}
	_, ok := reply.(*np.Rremove)
	if !ok {
		return np.MkErr(np.TErrBadFcall, "Rremovefile")
	}
	return nil
}

func (pclnt *ProtClnt) Clunk(fid np.Tfid) *np.Err {
	args := &np.Tclunk{fid}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	_, ok := reply.(*np.Rclunk)
	if !ok {
		return np.MkErr(np.TErrBadFcall, "Rclunk")
	}
	return nil
}

func (pclnt *ProtClnt) Open(fid np.Tfid, mode np.Tmode) (*np.Ropen, *np.Err) {
	args := &np.Topen{fid, mode}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Ropen)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Ropen")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Watch(fid np.Tfid) *np.Err {
	args := &np.Twatch{fid}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	_, ok := reply.(*np.Ropen)
	if !ok {
		return np.MkErr(np.TErrBadFcall, "Rwatch")
	}
	return nil
}

func (pclnt *ProtClnt) Read(fid np.Tfid, offset np.Toffset, cnt np.Tsize) (*np.Rread, *np.Err) {
	args := &np.Tread{fid, offset, cnt}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rread)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rread")
	}
	return msg, nil
}

func (pclnt *ProtClnt) ReadVF(fid np.Tfid, offset np.Toffset, cnt np.Tsize, f *np.Tfence, v np.TQversion) (*np.Rread, *np.Err) {
	args := &np.TreadV{fid, offset, cnt, v}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rread)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rread")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Write(fid np.Tfid, offset np.Toffset, data []byte) (*np.Rwrite, *np.Err) {
	args := &np.Twrite{fid, offset, data}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rwrite)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rwrite")
	}
	return msg, nil
}

func (pclnt *ProtClnt) WriteVF(fid np.Tfid, offset np.Toffset, f *np.Tfence, v np.TQversion, data []byte) (*np.Rwrite, *np.Err) {
	args := &np.TwriteV{fid, offset, v, data}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rwrite)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rwrite")
	}
	return msg, nil
}

func (pclnt *ProtClnt) WriteRead(fid np.Tfid, data []byte) (*np.Rwriteread, *np.Err) {
	args := &np.Twriteread{}
	args.Fid = uint64(fid)
	args.Data = data
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rwriteread)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rwriteread")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Stat(fid np.Tfid) (*np.Rstat, *np.Err) {
	args := &np.Tstat{fid}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rstat)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rstat")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Wstat(fid np.Tfid, st *np.Stat) (*np.Rwstat, *np.Err) {
	args := &np.Twstat{fid, 0, *st}
	reply, err := pclnt.CallNoFence(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rwstat)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rwstat")
	}
	return msg, nil
}

func (pclnt *ProtClnt) WstatF(fid np.Tfid, st *np.Stat, f *np.Tfence) (*np.Rwstat, *np.Err) {
	args := &np.Twstat{fid, 0, *st}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rwstat)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rwstat")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Renameat(oldfid np.Tfid, oldname string, newfid np.Tfid, newname string, f *np.Tfence) (*np.Rrenameat, *np.Err) {
	args := &np.Trenameat{oldfid, oldname, newfid, newname}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rrenameat)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rrenameat")
	}
	return msg, nil
}

func (pclnt *ProtClnt) GetFile(fid np.Tfid, path np.Path, mode np.Tmode, offset np.Toffset, cnt np.Tsize, resolve bool, f *np.Tfence) (*np.Rgetfile, *np.Err) {
	args := &np.Tgetfile{fid, mode, offset, cnt, path, resolve}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rgetfile)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rgetfile")
	}
	return msg, nil
}

func (pclnt *ProtClnt) SetFile(fid np.Tfid, path np.Path, mode np.Tmode, offset np.Toffset, resolve bool, f *np.Tfence, data []byte) (*np.Rwrite, *np.Err) {
	args := &np.Tsetfile{fid, mode, offset, path, resolve, data}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rwrite)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rwrite")
	}
	return msg, nil
}

func (pclnt *ProtClnt) PutFile(fid np.Tfid, path np.Path, mode np.Tmode, perm np.Tperm, offset np.Toffset, f *np.Tfence, data []byte) (*np.Rwrite, *np.Err) {
	args := &np.Tputfile{fid, mode, perm, offset, path, data}
	reply, err := pclnt.Call(args, f)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(*np.Rwrite)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rwrite")
	}
	return msg, nil
}

func (pclnt *ProtClnt) Detach() *np.Err {
	args := &np.Tdetach{0, 0}
	_, err := pclnt.CallNoFence(args)
	if err != nil {
		return err
	}
	return nil
}
