package fsclnt

import (
	// "log"

	np "ulambda/ninep"
)

func (fsc *FsClient) attach(server string, fid np.Tfid, path []string) (*np.Rattach, error) {
	args := np.Tattach{np.NoTag, fid, np.NoFid, "fk", ""}
	var reply np.Rattach
	err := fsc.cm.makeCall(server, "FsConn.Attach", args, &reply)
	return &reply, err
}

func (fsc *FsClient) walk(fid np.Tfid, nfid np.Tfid, path []string) (*np.Rwalk, error) {
	args := np.Twalk{np.NoTag, fid, nfid, nil}
	var reply np.Rwalk
	err := fsc.cm.makeCall(fsc.fids[fid].server, "FsConn.Walk", args, &reply)
	return &reply, err
}

func (fsc *FsClient) create(fid np.Tfid, name string, perm np.Tperm, mode np.Tmode) (*np.Rcreate, error) {
	args := np.Tcreate{np.NoTag, fid, name, perm, mode}
	var reply np.Rcreate
	err := fsc.cm.makeCall(fsc.fids[fid].server, "FsConn.Create", args, &reply)
	return &reply, err
}

func (fsc *FsClient) clunk(fid np.Tfid) error {
	args := np.Tclunk{np.NoTag, fid}
	var reply np.Rclunk
	err := fsc.cm.makeCall(fsc.fids[fid].server, "FsConn.Clunk", args, &reply)
	return err
}
