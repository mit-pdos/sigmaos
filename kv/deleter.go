package kv

import (
	"path"
	"sync"

	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/fenceclnt1"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

//
// Delete a shard
//
// XXX cmd line utility rmdir
//

type Deleter struct {
	mu sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	fclnt    *fenceclnt1.FenceClnt
	epochstr string
}

func MakeDeleter(epochstr, sharddir string) (*Deleter, error) {
	dl := &Deleter{}
	dl.epochstr = epochstr
	dl.FsLib = fslib.MakeFsLib("deleter-" + proc.GetPid())
	dl.ProcClnt = procclnt.MakeProcClnt(dl.FsLib)
	crash.Crasher(dl.FsLib)
	err := dl.Started(proc.GetPid())

	if err := JoinEpoch(dl.FsLib, "KVDEL", epochstr, []string{KVDIR, path.Dir(sharddir)}); err != nil {
		dl.Exited(proc.GetPid(), proc.MakeStatusErr(err.Error(), nil))
		return nil, err
	}
	return dl, err
}

func (dl *Deleter) Delete(sharddir string) {
	db.DLPrintf("KVDEL", "conf %v delete %v\n", dl.epochstr, sharddir)

	// If sharddir isn't found, then an earlier delete succeeded;
	// we are done.
	if _, err := dl.Stat(sharddir); err != nil && np.IsErrNotfound(err) {
		db.DLPrintf("KVDEL_ERR", "Delete conf %v not found %v\n", dl.epochstr, sharddir)
		dl.Exited(proc.GetPid(), proc.MakeStatus(proc.StatusOK))
		return
	}

	if err := dl.RmDir(sharddir); err != nil {
		db.DLPrintf("KVDEL_ERR", "conf %v rmdir %v err %v\n", dl.epochstr, sharddir, err)
		dl.Exited(proc.GetPid(), proc.MakeStatusErr(err.Error(), nil))
	} else {
		dl.Exited(proc.GetPid(), proc.MakeStatus(proc.StatusOK))
	}
}
