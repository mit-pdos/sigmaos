package npclnt

import (
	"errors"
	"math/rand"
	"time"

	np "ulambda/ninep"
)

type NpClnt struct {
	session np.Tsession
	seqno   np.Tseqno
	cm      *ChanMgr
}

func MakeNpClnt() *NpClnt {
	npc := &NpClnt{}
	// Generate a fresh session token
	rand.Seed(time.Now().UnixNano())
	npc.session = np.Tsession(rand.Uint64())
	npc.seqno = 0
	npc.cm = makeChanMgr(npc.session, &npc.seqno)
	return npc
}

func (npc *NpClnt) Exit() {
	npc.cm.exit()
}

func (npc *NpClnt) CallServer(server []string, args np.Tmsg) (np.Tmsg, error) {
	reply, err := npc.cm.makeCall(server, args)
	if err != nil {
		return nil, err
	}
	rmsg, ok := reply.(np.Rerror)
	if ok {
		return nil, errors.New(rmsg.Ename)
	}
	return reply, nil
}

func (npc *NpClnt) Attach(server []string, uname string, fid np.Tfid, path []string) (*np.Rattach, error) {
	args := np.Tattach{fid, np.NoFid, uname, ""}
	reply, err := npc.CallServer(server, args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rattach)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}

type NpChan struct {
	server []string
	cm     *ChanMgr
}

func (npc *NpClnt) MakeNpChan(server []string) *NpChan {
	npchan := &NpChan{server, npc.cm}
	return npchan
}

func (npc *NpChan) Server() []string {
	return npc.server
}

func (npch *NpChan) Call(args np.Tmsg) (np.Tmsg, error) {
	reply, err := npch.cm.makeCall(npch.server, args)
	if err != nil {
		return nil, err
	}
	rmsg, ok := reply.(np.Rerror)
	if ok {
		return nil, errors.New(rmsg.Ename)
	}
	return reply, nil
}

func (npch *NpChan) Flush(tag np.Ttag) error {
	args := np.Tflush{tag}
	reply, err := npch.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Rflush)
	if !ok {
		return errors.New("Not correct reply msg")
	}
	return err
}

func (npch *NpChan) Walk(fid np.Tfid, nfid np.Tfid, path []string) (*np.Rwalk, error) {
	args := np.Twalk{fid, nfid, path}
	reply, err := npch.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rwalk)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}

func (npch *NpChan) Create(fid np.Tfid, name string, perm np.Tperm, mode np.Tmode) (*np.Rcreate, error) {
	args := np.Tcreate{fid, name, perm, mode}
	reply, err := npch.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rcreate)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}

func (npch *NpChan) Remove(fid np.Tfid) error {
	args := np.Tremove{fid}
	reply, err := npch.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Rremove)
	if !ok {
		return errors.New("Not correct reply msg")
	}
	return err
}

func (npch *NpChan) RemoveFile(fid np.Tfid, wnames []string) error {
	args := np.Tremovefile{fid, wnames}
	reply, err := npch.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Rremove)
	if !ok {
		return errors.New("Not correct reply msg")
	}
	return err
}

func (npch *NpChan) Clunk(fid np.Tfid) error {
	args := np.Tclunk{fid}
	reply, err := npch.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Rclunk)
	if !ok {
		return errors.New("Not correct reply msg")
	}
	return err
}

func (npch *NpChan) Open(fid np.Tfid, mode np.Tmode) (*np.Ropen, error) {
	args := np.Topen{fid, mode}
	reply, err := npch.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Ropen)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}

func (npch *NpChan) Watch(fid np.Tfid, path []string, version np.TQversion) error {
	args := np.Twatchv{fid, path, np.OWATCH, version}
	reply, err := npch.Call(args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Ropen)
	if !ok {
		return errors.New("Not correct reply msg")
	}
	return err
}

func (npch *NpChan) Read(fid np.Tfid, offset np.Toffset, cnt np.Tsize) (*np.Rread, error) {
	args := np.Tread{fid, offset, cnt}
	reply, err := npch.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rread)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}

func (npch *NpChan) Write(fid np.Tfid, offset np.Toffset, data []byte) (*np.Rwrite, error) {
	args := np.Twrite{fid, offset, data}
	reply, err := npch.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rwrite)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}

func (npch *NpChan) Stat(fid np.Tfid) (*np.Rstat, error) {
	args := np.Tstat{fid}
	reply, err := npch.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rstat)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}

func (npch *NpChan) Wstat(fid np.Tfid, st *np.Stat) (*np.Rwstat, error) {
	args := np.Twstat{fid, 0, *st}
	reply, err := npch.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rwstat)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}

func (npch *NpChan) Renameat(oldfid np.Tfid, oldname string, newfid np.Tfid, newname string) (*np.Rrenameat, error) {
	args := np.Trenameat{oldfid, oldname, newfid, newname}
	reply, err := npch.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rrenameat)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}

func (npch *NpChan) GetFile(fid np.Tfid, path []string, mode np.Tmode, offset np.Toffset, cnt np.Tsize) (*np.Rgetfile, error) {
	args := np.Tgetfile{fid, mode, offset, cnt, path}
	reply, err := npch.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rgetfile)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}

func (npch *NpChan) SetFile(fid np.Tfid, path []string, mode np.Tmode, perm np.Tperm, offset np.Toffset, version np.TQversion, data []byte) (*np.Rwrite, error) {
	args := np.Tsetfile{fid, mode, perm, version, offset, path, data}
	reply, err := npch.Call(args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rwrite)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}
