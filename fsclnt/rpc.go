package fsclnt

import (
	"errors"

	np "ulambda/ninep"
)

func (fsc *FsClient) call(fid np.Tfid, args np.Tmsg) (np.Tmsg, error) {
	reply, err := fsc.cm.makeCall(fsc.fids[fid].server, args)
	if err != nil {
		return nil, err
	}
	rmsg, ok := reply.(np.Rerror)
	if ok {
		return nil, errors.New(rmsg.Ename)
	}
	return reply, nil
}
func (fsc *FsClient) callServer(server string, args np.Tmsg) (np.Tmsg, error) {
	reply, err := fsc.cm.makeCall(server, args)
	if err != nil {
		return nil, err
	}
	rmsg, ok := reply.(np.Rerror)
	if ok {
		return nil, errors.New(rmsg.Ename)
	}
	return reply, nil
}

// XXX copying msg once too many?

func (fsc *FsClient) attach(server string, fid np.Tfid, path []string) (*np.Rattach, error) {
	args := np.Tattach{fid, np.NoFid, "fk", ""}
	reply, err := fsc.callServer(server, args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rattach)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err

}

func (fsc *FsClient) walk(fid np.Tfid, nfid np.Tfid, path []string) (*np.Rwalk, error) {
	args := np.Twalk{fid, nfid, path}
	reply, err := fsc.call(fid, args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rwalk)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}

func (fsc *FsClient) create(fid np.Tfid, name string, perm np.Tperm, mode np.Tmode) (*np.Rcreate, error) {
	args := np.Tcreate{fid, name, perm, mode}
	reply, err := fsc.call(fid, args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rcreate)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}

func (fsc *FsClient) mkpipe(fid np.Tfid, name string, perm np.Tperm) (*np.Rmkpipe, error) {
	args := np.Tmkpipe{fid, name, perm, 0}
	reply, err := fsc.call(fid, args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rmkpipe)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}

func (fsc *FsClient) open(fid np.Tfid, mode np.Tmode) (*np.Ropen, error) {
	args := np.Topen{fid, mode}
	reply, err := fsc.call(fid, args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Ropen)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}

func (fsc *FsClient) clunk(fid np.Tfid) error {
	args := np.Tclunk{fid}
	reply, err := fsc.call(fid, args)
	if err != nil {
		return err
	}
	_, ok := reply.(np.Rclunk)
	if !ok {
		return errors.New("Not correct reply msg")
	}
	return err
}

func (fsc *FsClient) read(fid np.Tfid, offset np.Toffset, cnt np.Tsize) (*np.Rread, error) {
	args := np.Tread{fid, offset, cnt}
	reply, err := fsc.call(fid, args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rread)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}

func (fsc *FsClient) write(fid np.Tfid, offset np.Toffset, data []byte) (*np.Rwrite, error) {
	args := np.Twrite{fid, offset, data}
	reply, err := fsc.call(fid, args)
	if err != nil {
		return nil, err
	}
	msg, ok := reply.(np.Rwrite)
	if !ok {
		return nil, errors.New("Not correct reply msg")
	}
	return &msg, err
}
