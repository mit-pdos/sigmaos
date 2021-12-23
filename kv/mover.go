package kv

import (
	"log"
	"sync"

	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/named"
	"ulambda/proc"
	"ulambda/procclnt"
)

// XXX cmd line utility cp

type Mover struct {
	mu sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
}

func MakeMover(docrash string) (*Mover, error) {
	mv := &Mover{}
	mv.FsLib = fslib.MakeFsLib("mover-" + proc.GetPid())
	mv.ProcClnt = procclnt.MakeProcClnt(mv.FsLib)
	if docrash == "YES" {
		crash.Crasher(mv.FsLib, 10)
	}
	mv.Started(proc.GetPid())
	return mv, nil
}

func shardDir(kvd string) string {
	return named.MEMFS + "/" + kvd
}

func shardPath(kvd, shard string) string {
	return named.MEMFS + "/" + kvd + "/shard" + shard
}

func keyPath(kvd, shard string, k string) string {
	d := shardPath(kvd, shard)
	return d + "/" + k
}

func shardTmp(shardp string) string {
	return shardp + "#"
}

// Move shard from src to dst
func (mv *Mover) moveShard(shard, src, dst string) error {
	s := shardPath(src, shard)
	d := shardPath(dst, shard)
	d1 := shardTmp(d)

	// An aborted view change may have created the directory and
	// partially copied files into it; remove it and start over.
	mv.RmDir(d1)

	// The previous mover might have crashed right after rename
	// below. If so, we are done and reuse d.
	_, err := mv.Stat(d)
	if err == nil {
		return nil
	}

	err = mv.Mkdir(d1, 0777)
	if err != nil {
		log.Printf("%v: Mkdir %v err %v\n", db.GetName(), d1, err)
		return err
	}
	db.DLPrintf("MV", "%v: Copy shard from %v to %v\n", db.GetName(), s, d1)
	err = mv.CopyDir(s, d1)
	if err != nil {
		log.Printf("%v: CopyDir shard%v to %v err %v\n", db.GetName(), s, d1, err)
		return err
	}
	log.Printf("%v: Copy shard%v to %v done\n", db.GetName(), s, d1)
	err = mv.Rename(d1, d)
	if err != nil {
		log.Printf("%v: Rename %v to %v err %v\n", db.GetName(), d1, d, err)
		return err
	}
	return nil
}

func (mv *Mover) Move(shard, src, dst string) {
	log.Printf("MV shard%v from %v to %v\n", shard, src, dst)
	err := mv.moveShard(shard, src, dst)
	if err != nil {
		log.Printf("MV moveShards shard%v %v %v err %v\n", shard, src, dst, err)
	}
	if err != nil {
		mv.Exited(proc.GetPid(), err.Error())
	} else {
		mv.Exited(proc.GetPid(), "OK")
	}
}
