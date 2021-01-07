package fsclnt

import (
	np "ulambda/ninep"
)

func (fsc *FsClient) attach(server string, fid np.Tfid, path []string) (*np.Rattach, error) {
	args := np.Tattach{fid, np.NoFid, "fk", ""}
	var reply np.Rattach
	err := fsc.cm.makeCall(server, "FsConn.Attach", args, &reply)
	return &reply, err
}

func (fsc *FsClient) walk(fid np.Tfid, nfid np.Tfid, path []string) (*np.Rwalk, error) {
	args := np.Twalk{fid, nfid, path}
	var reply np.Rwalk
	err := fsc.cm.makeCall(fsc.fids[fid].server, "FsConn.Walk", args, &reply)
	return &reply, err
}

func (fsc *FsClient) create(fid np.Tfid, name string, perm np.Tperm, mode np.Tmode) (*np.Rcreate, error) {
	args := np.Tcreate{fid, name, perm, mode}
	var reply np.Rcreate
	err := fsc.cm.makeCall(fsc.fids[fid].server, "FsConn.Create", args, &reply)
	return &reply, err
}

func (fsc *FsClient) mkdir(dfid np.Tfid, name string, mode np.Tmode) (*np.Rmkdir, error) {
	args := np.Tmkdir{dfid, name, mode, 0}
	var reply np.Rmkdir
	err := fsc.cm.makeCall(fsc.fids[dfid].server, "FsConn.Mkdir", args, &reply)
	return &reply, err
}

func (fsc *FsClient) symlink(fid np.Tfid, name string, target string) (*np.Rsymlink, error) {
	args := np.Tsymlink{fid, name, target, 0}
	var reply np.Rsymlink
	err := fsc.cm.makeCall(fsc.fids[fid].server, "FsConn.Symlink", args, &reply)
	return &reply, err
}

func (fsc *FsClient) mkpipe(fid np.Tfid, name string, mode np.Tmode) (*np.Rmkpipe, error) {
	args := np.Tmkpipe{fid, name, mode, 0}
	var reply np.Rmkpipe
	err := fsc.cm.makeCall(fsc.fids[fid].server, "FsConn.Pipe", args, &reply)
	return &reply, err
}

func (fsc *FsClient) readlink(fid np.Tfid) (*np.Rreadlink, error) {
	args := np.Treadlink{fid}
	var reply np.Rreadlink
	err := fsc.cm.makeCall(fsc.fids[fid].server, "FsConn.Readlink", args, &reply)
	return &reply, err
}

func (fsc *FsClient) open(fid np.Tfid, mode np.Tmode) (*np.Ropen, error) {
	args := np.Topen{fid, mode}
	var reply np.Ropen
	err := fsc.cm.makeCall(fsc.fids[fid].server, "FsConn.Open", args, &reply)
	return &reply, err
}

func (fsc *FsClient) clunk(fid np.Tfid) error {
	args := np.Tclunk{fid}
	var reply np.Rclunk
	err := fsc.cm.makeCall(fsc.fids[fid].server, "FsConn.Clunk", args, &reply)
	return err
}

func (fsc *FsClient) read(fid np.Tfid, offset np.Toffset, cnt np.Tsize) (*np.Rread, error) {
	args := np.Tread{fid, offset, cnt}
	var reply np.Rread
	err := fsc.cm.makeCall(fsc.fids[fid].server, "FsConn.Read", args, &reply)
	return &reply, err
}

func (fsc *FsClient) write(fid np.Tfid, offset np.Toffset, data []byte) (*np.Rwrite, error) {
	args := np.Twrite{fid, offset, data}
	var reply np.Rwrite
	err := fsc.cm.makeCall(fsc.fids[fid].server, "FsConn.Write", args, &reply)
	return &reply, err
}
