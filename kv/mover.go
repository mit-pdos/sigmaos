package kv

import (
	"fmt"
	"path"
	"strconv"
	"sync"

	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/fenceclnt"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/sessp"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

//
// Move shards between servers
//

type Mover struct {
	mu sync.Mutex
	*sigmaclnt.SigmaClnt
	fclnt    *fenceclnt.FenceClnt
	job      string
	epochstr string
	shard    uint32
	cc       *CacheClnt
}

func JoinEpoch(fsl *fslib.FsLib, job, label, epochstr string, dirs []string) error {
	fence, err := sessp.NewFenceJson([]byte(epochstr))
	if err != nil {
		return err
	}
	fclnt := fenceclnt.MakeFenceClnt(fsl)
	if err := fclnt.FenceAtEpoch(*fence, dirs); err != nil {
		return fmt.Errorf("FenceAtEpoch %v err %v", KVConfig(job), err)
	}
	// reads are fenced
	config := Config{}
	if err := fsl.GetFileJson(KVConfig(job), &config); err != nil {
		return fmt.Errorf("GetFileJson %v err %v", KVConfig(job), err)
	}
	if config.Fence.Epoch != fence.Epoch {
		return fmt.Errorf("Newer config %v", config.Fence)
	}
	return nil
}

func MakeMover(job, epochstr, shard, src, dst string) (*Mover, error) {
	sc, err := sigmaclnt.MkSigmaClnt(sp.Tuname("mover-" + proc.GetPid().String()))
	if err != nil {
		return nil, err
	}
	mv := &Mover{epochstr: epochstr, SigmaClnt: sc, job: job, cc: NewCacheClnt(sc.FsLib, NSHARD)}
	if sh, err := strconv.ParseUint(shard, 10, 32); err != nil {
		return nil, err
	} else {
		mv.shard = uint32(sh)
	}
	if err := mv.Started(); err != nil {
		db.DFatalf("%v: couldn't start %v\n", proc.GetName(), err)
	}
	crash.Crasher(mv.FsLib)

	if err := JoinEpoch(mv.FsLib, mv.job, "KVMV", epochstr, []string{JobDir(mv.job), path.Dir(src), path.Dir(dst)}); err != nil {
		mv.ClntExit(proc.MakeStatusErr(err.Error(), nil))
		return nil, err
	}
	return mv, nil
}

// Copy shard from src to dst
func (mv *Mover) moveShard(s, d string) error {
	if err := mv.cc.CreateShard(d, mv.shard); err != nil {
		return err
	}
	vals, err := mv.cc.DumpShard(s, mv.shard)
	if err != nil {
		return err
	}
	if err := mv.cc.FillShard(s, mv.shard, vals); err != nil {
		return err
	}
	if err := mv.cc.DeleteShard(s, mv.shard); err != nil {
		return err
	}
	return nil
}

func (mv *Mover) Move(src, dst string) {
	db.DPrintf(db.KVMV, "conf %v: mv %d from %v to %v\n", mv.epochstr, mv.shard, src, dst)
	err := mv.moveShard(src, dst)
	if err != nil {
		db.DPrintf(db.KVMV_ERR, "conf %v: mv %d from %v to %v err %v\n", mv.epochstr, mv.shard, src, dst, err)
	}
	db.DPrintf(db.KVMV, "conf %v: mv %d  done from %v to %v\n", mv.epochstr, mv.shard, src, dst)
	if err != nil {
		mv.ClntExit(proc.MakeStatusErr(err.Error(), nil))
	}
}
