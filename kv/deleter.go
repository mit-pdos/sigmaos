package kv

import (
	"fmt"
	"log"
	"strconv"
	"sync"

	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/fenceclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

// XXX cmd line utility rmdir

type Deleter struct {
	mu sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	fclnt  *fenceclnt.FenceClnt
	blConf Config
}

// XXX KV group from which we are deleting

func MakeDeleter(N string, sharddir string) (*Deleter, error) {
	dl := &Deleter{}
	dl.FsLib = fslib.MakeFsLib("deleter-" + proc.GetPid())
	dl.ProcClnt = procclnt.MakeProcClnt(dl.FsLib)
	crash.Crasher(dl.FsLib)
	err := dl.Started(proc.GetPid())
	dl.fclnt = fenceclnt.MakeFenceClnt(dl.FsLib, KVCONFIG, 0, []string{KVDIR})
	err = dl.fclnt.AcquireConfig(&dl.blConf)
	if err != nil {
		log.Printf("%v: fence %v err %v\n", proc.GetName(), dl.fclnt.Name(), err)
		return nil, err
	}
	if N != strconv.Itoa(dl.blConf.N) {
		log.Printf("%v: wrong config %v\n", proc.GetName(), N)
		return nil, fmt.Errorf("wrong config %v\n", N)
	}
	return dl, err
}

func (dl *Deleter) Delete(sharddir string) {
	db.DLPrintf("KVDEL", "conf %v delete %v\n", dl.blConf.N, sharddir)

	// If sharddir isn't found, then an earlier delete succeeded;
	// we are done.
	if _, err := dl.Stat(sharddir); err != nil && np.IsErrNotfound(err) {
		log.Printf("%v: Delete conf %v not found %v\n", proc.GetName(), dl.blConf.N, sharddir)
		dl.Exited(proc.GetPid(), proc.MakeStatus(proc.StatusOK))
		return
	}

	// Fence my writes to server holding sharddir
	if err := dl.fclnt.FencePaths([]string{sharddir}); err != nil {
		dl.Exited(proc.GetPid(), proc.MakeStatusErr(err.Error(), nil))
		return
	}

	if err := dl.RmDir(sharddir); err != nil {
		log.Printf("%v: conf %v rmdir %v err %v\n", proc.GetName(), dl.blConf.N, sharddir, err)
		dl.Exited(proc.GetPid(), proc.MakeStatusErr(err.Error(), nil))
	} else {
		dl.Exited(proc.GetPid(), proc.MakeStatus(proc.StatusOK))
	}
}
