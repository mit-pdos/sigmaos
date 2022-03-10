package kv

import (
	"fmt"
	"path"
	"strconv"
	"sync"

	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/fenceclnt"
	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procclnt"
)

// XXX cmd line utility cp

type Mover struct {
	mu sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	fclnt  *fenceclnt.FenceClnt
	blConf Config
}

func MakeMover(N string, src, dst string) (*Mover, error) {
	mv := &Mover{}
	mv.FsLib = fslib.MakeFsLib("mover-" + proc.GetPid())
	mv.ProcClnt = procclnt.MakeProcClnt(mv.FsLib)

	err := mv.Started(proc.GetPid())
	crash.Crasher(mv.FsLib)

	mv.fclnt = fenceclnt.MakeFenceClnt(mv.FsLib, KVCONFIG, 0, []string{KVDIR})

	err = mv.fclnt.AcquireConfig(&mv.blConf)
	if err != nil {
		db.DLPrintf("KVMV_ERR", "AcquireConfig %v err %v\n", mv.fclnt.Name(), err)
		return nil, err
	}
	if N != strconv.Itoa(mv.blConf.N) {
		return nil, fmt.Errorf("wrong config %v", N)
	}
	return mv, err
}

func shardTmp(shardp string) string {
	return shardp + "#"
}

// Move shard from src to dst
func (mv *Mover) moveShard(s, d string) error {
	d1 := shardTmp(d)

	// The previous mover might have crashed right after rename
	// below. If so, we are done.
	_, err := mv.Stat(d)
	if err == nil {
		db.DLPrintf("KVMV_ERR", "moveShard conf %v exists %v\n", mv.blConf.N, d)
		return nil
	}

	// Fence writes to server holding d.  Also alert s about the
	// new config so that fenced writes from clerks so s will
	// fail.
	//
	// + "/" to force grp-0 to be resolved and path.Dir(d) because
	// d doesn't exist yet.
	if err := mv.fclnt.FencePaths([]string{s, path.Dir(d) + "/"}); err != nil {
		return err
	}

	// An aborted view change may have created the directory and
	// partially copied files into it; remove it and start over.
	mv.RmDir(d1)

	err = mv.MkDir(d1, 0777)
	if err != nil {
		db.DLPrintf("KVMV_ERR", "Mkdir %v err %v\n", d1, err)
		return err
	}
	// log.Printf("%v: Copy shard from %v to %v\n", proc.GetName(), s, d1)
	err = mv.CopyDir(s, d1)
	if err != nil {
		db.DLPrintf("KVMV_ERR", "CopyDir shard%v to %v err %v\n", s, d1, err)
		return err
	}
	// log.Printf("%v: Copy shard%v to %v done\n", proc.GetName(), s, d1)
	err = mv.Rename(d1, d)
	if err != nil {
		db.DLPrintf("KVMV_ERR", "Rename %v to %v err %v\n", d1, d, err)
		return err
	}
	return nil
}

func (mv *Mover) Move(src, dst string) {
	db.DLPrintf("KVMV", "conf %v: mv from %v to %v\n", mv.blConf.N, src, dst)
	err := mv.moveShard(src, dst)
	if err != nil {
		db.DLPrintf("KVMV_ERR", "conf %v from %v to %v err %v\n", mv.blConf.N, src, dst, err)
	}
	if err != nil {
		mv.Exited(proc.GetPid(), proc.MakeStatusErr(err.Error(), nil))
	} else {
		mv.Exited(proc.GetPid(), proc.MakeStatus(proc.StatusOK))
	}
}
