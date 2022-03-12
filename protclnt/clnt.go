package protclnt

import (
	np "ulambda/ninep"
	"ulambda/rand"
)

type Clnt struct {
	session np.Tsession
	seqno   np.Tseqno
	cm      *ConnMgr
}

func MakeClnt() *Clnt {
	clnt := &Clnt{}
	clnt.session = np.Tsession(rand.Uint64())
	clnt.seqno = 0
	clnt.cm = makeConnMgr(clnt.session, &clnt.seqno)
	return clnt
}

func (clnt *Clnt) ReadSeqNo() np.Tseqno {
	return clnt.seqno
}

func (clnt *Clnt) Exit() {
	clnt.cm.exit()
}

func (clnt *Clnt) CallServer(server []string, args np.Tmsg) (np.Tmsg, *np.Err) {
	reply, err := clnt.cm.makeCall(server, args)
	if err != nil {
		return nil, err
	}
	rmsg, ok := reply.(np.Rerror)
	if ok {
		return nil, np.String2Err(rmsg.Ename)
	}
	return reply, nil
}

func (clnt *Clnt) Attach(server []string, uname string, fid np.Tfid, path np.Path) (*np.Rattach, *np.Err) {
	args := np.Tattach{fid, np.NoFid, uname, path.String()}
	reply, err := clnt.CallServer(server, args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rattach)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "clnt")
	}
	return &msg, nil
}

func (clnt *Clnt) MakeProtClnt(server []string) *ProtClnt {
	protclnt := &ProtClnt{server, clnt}
	return protclnt
}

type ProtClnt struct {
	server []string
	clnt   *Clnt
}

func (pclnt *ProtClnt) Server() []string {
	return pclnt.server
}

func (pclnt *ProtClnt) Disconnect() *np.Err {
	return pclnt.clnt.cm.disconnect(pclnt.server)
}

func (pclnt *ProtClnt) Call(args np.Tmsg) (np.Tmsg, *np.Err) {
	return pclnt.clnt.CallServer(pclnt.server, args)
}

func (pclnt *ProtClnt) Flush(tag np.Ttag) *np.Err {
	args := np.Tflush{tag}
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Rflush)
	if !ok {
		return np.MkErr(np.TErrBadFcall, "Rflush")
	}
	return nil
}

func (pclnt *ProtClnt) Walk(fid np.Tfid, nfid np.Tfid, path np.Path) (*np.Rwalk, *np.Err) {
	args := np.Twalk{fid, nfid, path}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rwalk)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rwalk")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) Create(fid np.Tfid, name string, perm np.Tperm, mode np.Tmode) (*np.Rcreate, *np.Err) {
	args := np.Tcreate{fid, name, perm, mode}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rcreate)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rcreate")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) Remove(fid np.Tfid) *np.Err {
	args := np.Tremove{fid}
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Rremove)
	if !ok {
		return np.MkErr(np.TErrBadFcall, "Rremove")
	}
	return nil
}

func (pclnt *ProtClnt) RemoveFile(fid np.Tfid, wnames np.Path, resolve bool) *np.Err {
	args := np.Tremovefile{fid, wnames, resolve}
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Rremove)
	if !ok {
		return np.MkErr(np.TErrBadFcall, "Rremovefile")
	}
	return nil
}

func (pclnt *ProtClnt) Clunk(fid np.Tfid) *np.Err {
	args := np.Tclunk{fid}
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Rclunk)
	if !ok {
		return np.MkErr(np.TErrBadFcall, "Rclunk")
	}
	return nil
}

func (pclnt *ProtClnt) Open(fid np.Tfid, mode np.Tmode) (*np.Ropen, *np.Err) {
	args := np.Topen{fid, mode}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Ropen)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Ropen")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) Watch(fid np.Tfid, path np.Path) *np.Err {
	args := np.Twatch{fid, path}
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Ropen)
	if !ok {
		return np.MkErr(np.TErrBadFcall, "Rwatch")
	}
	return nil
}

func (pclnt *ProtClnt) Read(fid np.Tfid, offset np.Toffset, cnt np.Tsize) (*np.Rread, *np.Err) {
	args := np.Tread{fid, offset, cnt}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rread)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rread")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) Write(fid np.Tfid, offset np.Toffset, data []byte) (*np.Rwrite, *np.Err) {
	args := np.Twrite{fid, offset, data}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rwrite)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rwrite")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) Stat(fid np.Tfid) (*np.Rstat, *np.Err) {
	args := np.Tstat{fid}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, nil
	}
	msg, ok := reply.(np.Rstat)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rstat")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) Wstat(fid np.Tfid, st *np.Stat) (*np.Rwstat, *np.Err) {
	args := np.Twstat{fid, 0, *st}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rwstat)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rwstat")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) Renameat(oldfid np.Tfid, oldname string, newfid np.Tfid, newname string) (*np.Rrenameat, *np.Err) {
	args := np.Trenameat{oldfid, oldname, newfid, newname}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rrenameat)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rrenameat")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) GetFile(fid np.Tfid, path np.Path, mode np.Tmode, offset np.Toffset, cnt np.Tsize, resolve bool) (*np.Rgetfile, *np.Err) {
	args := np.Tgetfile{fid, mode, offset, cnt, path, resolve}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rgetfile)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rgetfile")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) SetFile(fid np.Tfid, path np.Path, mode np.Tmode, offset np.Toffset, data []byte, resolve bool) (*np.Rwrite, *np.Err) {
	args := np.Tsetfile{fid, mode, offset, path, resolve, data}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rwrite)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rwrite")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) PutFile(fid np.Tfid, path np.Path, mode np.Tmode, perm np.Tperm, offset np.Toffset, data []byte) (*np.Rwrite, *np.Err) {
	args := np.Tputfile{fid, mode, perm, offset, path, data}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rwrite)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rwrite")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) MkFence(fid np.Tfid) (*np.Rmkfence, *np.Err) {
	args := np.Tmkfence{fid, np.NoSeqno}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rmkfence)
	if !ok {
		return nil, np.MkErr(np.TErrBadFcall, "Rmkfence")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) RegisterFence(fence np.Tfence, fid np.Tfid) *np.Err {
	args := np.Tregfence{fid, fence}
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Ropen)
	if !ok {
		return np.MkErr(np.TErrBadFcall, "RegisterFence")
	}
	return nil
}

func (pclnt *ProtClnt) DeregisterFence(fence np.Tfence, fid np.Tfid) *np.Err {
	args := np.Tunfence{fid, fence}
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Ropen)
	if !ok {
		return np.MkErr(np.TErrBadFcall, "DeregisterFence")
	}
	return nil
}

func (pclnt *ProtClnt) RmFence(fence np.Tfence, fid np.Tfid) *np.Err {
	args := np.Trmfence{fid, fence}
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Ropen)
	if !ok {
		return np.MkErr(np.TErrBadFcall, "DeregisterFence")
	}
	return nil
}
