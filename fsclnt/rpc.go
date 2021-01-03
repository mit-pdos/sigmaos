package fsclnt

import (
	np "ulambda/ninep"
)

func (fsc *FsClient) attach(server string, fid np.Tfid, path []string) (*np.Rattach, error) {
	args := np.Tattach{np.NoTag, fid, np.NoFid, "fk", ""}
	var reply np.Rattach
	err := fsc.cm.makeCall(server, "Client.Attach", args, &reply)
	return &reply, err
}

func (fsc *FsClient) walk(fid np.Tfid, nfid np.Tfid, path []string) (*np.Rwalk, error) {
	args := np.Twalk{np.NoTag, fid, nfid, path}
	var reply np.Rwalk
	err := fsc.cm.makeCall(fsc.fids[fid].server, "Client.Walk", args, &reply)
	return &reply, err
}

func (fsc *FsClient) create(fid np.Tfid, name string, perm np.Tperm, mode np.Tmode) (*np.Rcreate, error) {
	args := np.Tcreate{np.NoTag, fid, name, perm, mode}
	var reply np.Rcreate
	err := fsc.cm.makeCall(fsc.fids[fid].server, "Client.Create", args, &reply)
	return &reply, err
}

func (fsc *FsClient) symlink(fid np.Tfid, name string, target string) (*np.Rsymlink, error) {
	args := np.Tsymlink{np.NoTag, fid, name, target, 0}
	var reply np.Rsymlink
	err := fsc.cm.makeCall(fsc.fids[fid].server, "Client.Symlink", args, &reply)
	return &reply, err
}

func (fsc *FsClient) readlink(fid np.Tfid) (*np.Rreadlink, error) {
	args := np.Treadlink{np.NoTag, fid}
	var reply np.Rreadlink
	err := fsc.cm.makeCall(fsc.fids[fid].server, "Client.Readlink", args, &reply)
	return &reply, err
}

func (fsc *FsClient) open(fid np.Tfid, mode np.Tmode) (*np.Ropen, error) {
	args := np.Topen{np.NoTag, fid, mode}
	var reply np.Ropen
	err := fsc.cm.makeCall(fsc.fids[fid].server, "Client.Open", args, &reply)
	return &reply, err
}

func (fsc *FsClient) clunk(fid np.Tfid) error {
	args := np.Tclunk{np.NoTag, fid}
	var reply np.Rclunk
	err := fsc.cm.makeCall(fsc.fids[fid].server, "Client.Clunk", args, &reply)
	return err
}

func (fsc *FsClient) read(fid np.Tfid, offset np.Toffset, cnt np.Tsize) (*np.Rread, error) {
	args := np.Tread{np.NoTag, fid, offset, cnt}
	var reply np.Rread
	err := fsc.cm.makeCall(fsc.fids[fid].server, "Client.Read", args, &reply)
	return &reply, err
}

func (fsc *FsClient) write(fid np.Tfid, offset np.Toffset, data []byte) (*np.Rwrite, error) {
	args := np.Twrite{np.NoTag, fid, offset, data}
	var reply np.Rwrite
	err := fsc.cm.makeCall(fsc.fids[fid].server, "Client.Write", args, &reply)
	return &reply, err
}
