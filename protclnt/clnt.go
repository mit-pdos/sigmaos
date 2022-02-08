package protclnt

import (
	"errors"

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

func (clnt *Clnt) RegisterFence(fence np.Tfence, new bool) error {
	return clnt.cm.registerFence(fence, new)
}

func (clnt *Clnt) DeregisterFence(fence np.Tfence) error {
	return clnt.cm.deregisterFence(fence)
}

func (clnt *Clnt) RmFence(fence np.Tfence) error {
	return clnt.cm.rmFence(fence)
}

func (clnt *Clnt) CallServer(server []string, args np.Tmsg) (np.Tmsg, *np.Err) {
	reply, err := clnt.cm.makeCall(server, args)
	if err != nil {
		return nil, err
	}
	rmsg, ok := reply.(np.Rerror)
	if ok {
		return nil, np.Error2Err(rmsg.Ename)
	}
	return reply, nil
}

func (clnt *Clnt) Attach(server []string, uname string, fid np.Tfid, path []string) (*np.Rattach, error) {
	args := np.Tattach{fid, np.NoFid, uname, np.Join(path)}
	reply, err := clnt.CallServer(server, args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rattach)
	if !ok {
		return nil, errors.New("Not correct reply msg")
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

func (pclnt *ProtClnt) Disconnect() {
	pclnt.clnt.cm.disconnect(pclnt.server)
}

func (pclnt *ProtClnt) Call(args np.Tmsg) (np.Tmsg, *np.Err) {
	return pclnt.clnt.CallServer(pclnt.server, args)
}

func (pclnt *ProtClnt) Flush(tag np.Ttag) error {
	args := np.Tflush{tag}
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Rflush)
	if !ok {
		return errors.New("Not correct reply msg")
	}
	return nil
}

func (pclnt *ProtClnt) Walk(fid np.Tfid, nfid np.Tfid, path []string) (*np.Rwalk, error) {
	args := np.Twalk{fid, nfid, path}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rwalk)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) Create(fid np.Tfid, name string, perm np.Tperm, mode np.Tmode) (*np.Rcreate, error) {
	args := np.Tcreate{fid, name, perm, mode}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rcreate)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) Remove(fid np.Tfid) error {
	args := np.Tremove{fid}
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Rremove)
	if !ok {
		return errors.New("Not correct reply msg")
	}
	return nil
}

func (pclnt *ProtClnt) RemoveFile(fid np.Tfid, wnames []string) error {
	args := np.Tremovefile{fid, wnames}
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Rremove)
	if !ok {
		return errors.New("Not correct reply msg")
	}
	return nil
}

func (pclnt *ProtClnt) Clunk(fid np.Tfid) error {
	args := np.Tclunk{fid}
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Rclunk)
	if !ok {
		return errors.New("Not correct reply msg")
	}
	return nil
}

func (pclnt *ProtClnt) Open(fid np.Tfid, mode np.Tmode) (*np.Ropen, error) {
	args := np.Topen{fid, mode}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Ropen)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) Watch(fid np.Tfid, path []string, version np.TQversion) error {
	args := np.Twatchv{fid, path, np.OWATCH, version}
	reply, err := pclnt.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Ropen)
	if !ok {
		return errors.New("Not correct reply msg")
	}
	return nil
}

func (pclnt *ProtClnt) Read(fid np.Tfid, offset np.Toffset, cnt np.Tsize) (*np.Rread, error) {
	args := np.Tread{fid, offset, cnt}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rread)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) Write(fid np.Tfid, offset np.Toffset, data []byte) (*np.Rwrite, error) {
	args := np.Twrite{fid, offset, data}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rwrite)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) Stat(fid np.Tfid) (*np.Rstat, error) {
	args := np.Tstat{fid}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, nil
	}
	msg, ok := reply.(np.Rstat)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) Wstat(fid np.Tfid, st *np.Stat) (*np.Rwstat, error) {
	args := np.Twstat{fid, 0, *st}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rwstat)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) Renameat(oldfid np.Tfid, oldname string, newfid np.Tfid, newname string) (*np.Rrenameat, error) {
	args := np.Trenameat{oldfid, oldname, newfid, newname}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rrenameat)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) GetFile(fid np.Tfid, path []string, mode np.Tmode, offset np.Toffset, cnt np.Tsize) (*np.Rgetfile, error) {
	args := np.Tgetfile{fid, mode, offset, cnt, path}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rgetfile)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) SetFile(fid np.Tfid, path []string, mode np.Tmode, perm np.Tperm, offset np.Toffset, data []byte) (*np.Rwrite, error) {
	args := np.Tsetfile{fid, mode, perm, offset, path, data}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rwrite)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, nil
}

func (pclnt *ProtClnt) MkFence(fid np.Tfid) (*np.Rmkfence, error) {
	args := np.Tmkfence{fid, np.NoSeqno}
	reply, err := pclnt.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rmkfence)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, nil
}
