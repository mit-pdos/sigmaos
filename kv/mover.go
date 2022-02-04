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

func MakeMover(N string) (*Mover, error) {
	mv := &Mover{}
	mv.FsLib = fslib.MakeFsLib("mover-" + proc.GetPid())
	mv.ProcClnt = procclnt.MakeProcClnt(mv.FsLib)
	mv.fclnt = fenceclnt.MakeFenceClnt(mv.FsLib, KVCONFIG, 0)
	err := mv.Started(proc.GetPid())
	crash.Crasher(mv.FsLib)
	err = mv.fclnt.AcquireConfig(&mv.blConf)
	if err != nil {
		log.Printf("%v: fence %v err %v\n", db.GetName(), mv.fclnt.Name(), err)
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

	// An aborted view change may have created the directory and
	// partially copied files into it; remove it and start over.
	mv.RmDir(d1)

	// The previous mover might have crashed right after rename
	// below. If so, we are done and reuse d.
	_, err := mv.Stat(d)
	if err == nil {
		log.Printf("%v: moveShard conf %v reuse %v\n", db.GetName(), mv.blConf.N, d)
		return nil
	}

	err = mv.Mkdir(d1, 0777)
	if err != nil {
		//log.Printf("%v: Mkdir %v err %v\n", db.GetName(), d1, err)
		return err
	}
	// log.Printf("%v: Copy shard from %v to %v\n", db.GetName(), s, d1)
	err = mv.CopyDir(s, d1)
	if err != nil {
		//log.Printf("%v: CopyDir shard%v to %v err %v\n", db.GetName(), s, d1, err)
		return err
	}
	// log.Printf("%v: Copy shard%v to %v done\n", db.GetName(), s, d1)
	err = mv.Rename(d1, d)
	if err != nil {
		//log.Printf("%v: Rename %v to %v err %v\n", db.GetName(), d1, d, err)
		return err
	}
	return nil
}

func (mv *Mover) Move(src, dst string) {
	// log.Printf("%v: MV conf %v from %v to %v\n", db.GetName(), mv.blConf.N, src, dst)
	err := mv.moveShard(src, dst)
	if err != nil {
		log.Printf("%v: MV conf %v from %v to %v err %v\n", db.GetName(), mv.blConf.N, src, dst, err)
	}
	if err != nil {
		mv.Exited(proc.GetPid(), err.Error())
	} else {
		mv.Exited(proc.GetPid(), "OK")
	}
}
