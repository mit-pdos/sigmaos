// Package spprotoclnt implements stubs for the sigmap messages for a
// server at an endpoint. It relies on [sessclnt] for maintaining a
// session with that server.
package clnt

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sessclnt "sigmaos/session/clnt"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
)

type SPProtoClnt struct {
	ep *sp.Tendpoint
	sm *sessclnt.Mgr
}

func NewSPProtoClnt(ep *sp.Tendpoint, sm *sessclnt.Mgr) *SPProtoClnt {
	return &SPProtoClnt{
		ep: ep,
		sm: sm,
	}
}

func (pclnt *SPProtoClnt) Endpoint() *sp.Tendpoint {
	return pclnt.ep
}

func (pclnt *SPProtoClnt) CallServer(args sessp.Tmsg, iniov sessp.IoVec, outiov sessp.IoVec) (*sessp.FcallMsg, *serr.Err) {
	reply, err := pclnt.sm.RPC(pclnt.ep, args, iniov, outiov)
	if err != nil {
		return nil, err
	}
	rmsg, ok := reply.Msg.(*sp.Rerror)
	if ok {
		return nil, sp.NewErr(rmsg)
	}
	return reply, nil
}

func (pclnt *SPProtoClnt) Call(args sessp.Tmsg) (*sessp.FcallMsg, *serr.Err) {
	return pclnt.CallServer(args, nil, nil)
}

func (pclnt *SPProtoClnt) CallIoVec(args sessp.Tmsg, iniov sessp.IoVec, outiov sessp.IoVec) (*sessp.FcallMsg, *serr.Err) {
	return pclnt.CallServer(args, iniov, outiov)
}

func (pclnt *SPProtoClnt) Attach(secrets map[string]*sp.SecretProto, cid sp.TclntId, fid sp.Tfid, path path.Tpathname) (*sp.Rattach, *serr.Err) {
	args := sp.NewTattach(fid, sp.NoFid, secrets, cid, path)
	reply, err := pclnt.CallServer(args, nil, nil)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rattach)
	if !ok {
		return nil, serr.NewErr(serr.TErrBadFcall, "clnt")
	}
	db.DPrintf(db.PROTCLNT, "Attach path %v cid %v sessid %v", path, cid, reply.Fc.Session)
	return msg, nil
}

func (pclnt *SPProtoClnt) Walk(fid sp.Tfid, nfid sp.Tfid, path path.Tpathname) (*sp.Rwalk, *serr.Err) {
	args := sp.NewTwalk(fid, nfid, path)
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rwalk)
	if !ok {
		return nil, serr.NewErr(serr.TErrBadFcall, "Rwalk")
	}
	return msg, nil
}

func (pclnt *SPProtoClnt) Create(fid sp.Tfid, name string, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f *sp.Tfence) (*sp.Rcreate, *serr.Err) {
	args := sp.NewTcreate(fid, name, perm, mode, lid, f)
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rcreate)
	if !ok {
		return nil, serr.NewErr(serr.TErrBadFcall, "Rcreate")
	}
	return msg, nil
}

func (pclnt *SPProtoClnt) Remove(fid sp.Tfid) *serr.Err {
	args := sp.NewTremove(fid, sp.NullFence())
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.Msg.(*sp.Rremove)
	if !ok {
		return serr.NewErr(serr.TErrBadFcall, "Rremove")
	}
	return nil
}

func (pclnt *SPProtoClnt) RemoveF(fid sp.Tfid, f *sp.Tfence) *serr.Err {
	args := sp.NewTremove(fid, f)
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.Msg.(*sp.Rremove)
	if !ok {
		return serr.NewErr(serr.TErrBadFcall, "Rremove")
	}
	return nil
}

func (pclnt *SPProtoClnt) RemoveFile(fid sp.Tfid, wnames path.Tpathname, resolve bool, f *sp.Tfence) *serr.Err {
	args := sp.NewTremovefile(fid, wnames, resolve, f)
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.Msg.(*sp.Rremove)
	if !ok {
		return serr.NewErr(serr.TErrBadFcall, "Rremovefile")
	}
	return nil
}

func (pclnt *SPProtoClnt) Clunk(fid sp.Tfid) *serr.Err {
	args := sp.NewTclunk(fid)
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.Msg.(*sp.Rclunk)
	if !ok {
		return serr.NewErr(serr.TErrBadFcall, "Rclunk")
	}
	return nil
}

func (pclnt *SPProtoClnt) Open(fid sp.Tfid, mode sp.Tmode) (*sp.Ropen, *serr.Err) {
	args := sp.NewTopen(fid, mode)
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Ropen)
	if !ok {
		return nil, serr.NewErr(serr.TErrBadFcall, "Ropen")
	}
	return msg, nil
}

func (pclnt *SPProtoClnt) Watch(fid sp.Tfid) *serr.Err {
	args := sp.NewTwatch(fid)
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.Msg.(*sp.Ropen)
	if !ok {
		return serr.NewErr(serr.TErrBadFcall, "Rwatch")
	}
	return nil
}

func (pclnt *SPProtoClnt) ReadF(fid sp.Tfid, offset sp.Toffset, b []byte, f *sp.Tfence) (sp.Tsize, *serr.Err) {
	args := sp.NewReadF(fid, offset, sp.Tsize(len(b)), f)
	reply, err := pclnt.CallIoVec(args, nil, sessp.IoVec{b})
	if err != nil {
		return 0, err
	}
	rep, ok := reply.Msg.(*sp.Rread)
	if !ok {
		return 0, serr.NewErr(serr.TErrBadFcall, "Rread")
	}
	db.DPrintf(db.PROTCLNT, "Read fid %v cnt %v", fid, rep.Tcount())
	return rep.Tcount(), nil
}

func (pclnt *SPProtoClnt) WriteF(fid sp.Tfid, offset sp.Toffset, f *sp.Tfence, data []byte) (*sp.Rwrite, *serr.Err) {
	args := sp.NewTwriteF(fid, offset, f)
	reply, err := pclnt.CallIoVec(args, sessp.IoVec{data}, nil)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rwrite)
	if !ok {
		return nil, serr.NewErr(serr.TErrBadFcall, "Rwrite")
	}
	return msg, nil
}

func (pclnt *SPProtoClnt) WriteRead(fid sp.Tfid, iniov sessp.IoVec, outiov sessp.IoVec) *serr.Err {
	args := sp.NewTwriteread(fid)
	reply, err := pclnt.CallIoVec(args, iniov, outiov)
	if err != nil {
		return err
	}
	_, ok := reply.Msg.(*sp.Rread)
	if !ok {
		return serr.NewErr(serr.TErrBadFcall, "Rwriteread")
	}
	if len(outiov) != len(reply.Iov) {
		// Sanity check: if the caller supplied IoVecs to write outputs to, ensure
		// that they supplied at least enough of them. In the event that
		// the result of the RPC is an error, we may get the case that
		// len(iov) < fm.Fc.Nvec
		if len(outiov) < len(reply.Iov) {
			return serr.NewErr(serr.TErrBadFcall, fmt.Sprintf("protclnt outiov len insufficient: prov %v != %v res", len(outiov), len(reply.Iov)))
		}
	}
	return nil
}

func (pclnt *SPProtoClnt) Stat(fid sp.Tfid) (*sp.Rrstat, *serr.Err) {
	args := sp.NewTrstat(fid)
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rrstat)
	if !ok {
		return nil, serr.NewErr(serr.TErrBadFcall, "Rstat")
	}
	return msg, nil
}

func (pclnt *SPProtoClnt) Wstat(fid sp.Tfid, st *sp.Tstat) (*sp.Rwstat, *serr.Err) {
	args := sp.NewTwstat(fid, st, sp.NullFence())
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rwstat)
	if !ok {
		return nil, serr.NewErr(serr.TErrBadFcall, "Rwstat")
	}
	return msg, nil
}

func (pclnt *SPProtoClnt) WstatF(fid sp.Tfid, st *sp.Tstat, f *sp.Tfence) (*sp.Rwstat, *serr.Err) {
	args := sp.NewTwstat(fid, st, f)
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rwstat)
	if !ok {
		return nil, serr.NewErr(serr.TErrBadFcall, "Rwstat")
	}
	return msg, nil
}

func (pclnt *SPProtoClnt) Renameat(oldfid sp.Tfid, oldname string, newfid sp.Tfid, newname string, f *sp.Tfence) (*sp.Rrenameat, *serr.Err) {
	args := sp.NewTrenameat(oldfid, oldname, newfid, newname, f)
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rrenameat)
	if !ok {
		return nil, serr.NewErr(serr.TErrBadFcall, "Rrenameat")
	}
	return msg, nil
}

func (pclnt *SPProtoClnt) GetFile(fid sp.Tfid, path path.Tpathname, mode sp.Tmode, offset sp.Toffset, cnt sp.Tsize, resolve bool, f *sp.Tfence) ([]byte, *serr.Err) {
	args := sp.NewTgetfile(fid, mode, offset, cnt, path, resolve, f)
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	_, ok := reply.Msg.(*sp.Rread)
	if !ok {
		return nil, serr.NewErr(serr.TErrBadFcall, "Rgetfile")
	}
	return reply.Iov[0], nil
}

func (pclnt *SPProtoClnt) PutFile(fid sp.Tfid, path path.Tpathname, mode sp.Tmode, perm sp.Tperm, offset sp.Toffset, resolve bool, f *sp.Tfence, data []byte, lid sp.TleaseId) (*sp.Rwrite, *serr.Err) {
	args := sp.NewTputfile(fid, mode, perm, offset, path, resolve, lid, f)
	reply, err := pclnt.CallIoVec(args, sessp.IoVec{data}, nil)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.Msg.(*sp.Rwrite)
	if !ok {
		return nil, serr.NewErr(serr.TErrBadFcall, "Rwrite")
	}
	return msg, nil
}

func (pclnt *SPProtoClnt) Detach(cid sp.TclntId) *serr.Err {
	args := sp.NewTdetach(cid)
	_, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	return nil
}
