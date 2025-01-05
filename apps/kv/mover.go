package kv

import (
	"strconv"
	"sync"
	"time"

	"sigmaos/apps/cache"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/util/crash"
)

//
// Move shards between servers
//

type Mover struct {
	mu sync.Mutex
	*sigmaclnt.SigmaClnt
	job   string
	fence *sp.Tfence
	shard cache.Tshard
	kc    *KvClerk
	exit  bool
}

func checkFence(fsl *fslib.FsLib, job string, fence *sp.Tfence) {
	config := Config{}
	if err := fsl.GetFileJson(KVConfig(job), &config); err != nil {
		db.DPrintf(db.ALWAYS, "checkFence: GetFile err %v\n", err)
	}
	if fence.LessThan(&config.Fence) {
		db.DPrintf(db.ALWAYS, "checkFence: Mover is stale %v %v\n", fence, config.Fence)
	}
}

func NewMover(job, epochstr, shard, src, dst, repl string) (*Mover, error) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	fence, err := sp.NewFenceJson([]byte(epochstr))
	if err != nil {
		return nil, err
	}
	mv := &Mover{fence: fence,
		SigmaClnt: sc,
		job:       job,
		kc:        NewClerkFsLib(sc.FsLib, job, repl == "repl"),
		exit:      true,
	}
	if sh, err := strconv.ParseUint(shard, 10, 32); err != nil {
		return nil, err
	} else {
		mv.shard = cache.Tshard(sh)
	}
	if err := mv.Started(); err != nil {
		db.DFatalf("couldn't start %v", err)
	}

	crash.Failer(mv.FsLib, crash.KVMOVER_EVENT, func(e crash.Tevent) {
		if crash.Rand50() {
			crash.Crash()
		} else {
			// Randomly tell parent we exited but then keep running,
			// simulating a network partition from the parent's point
			// of view.
			db.DPrintf(db.CRASH, "Partition mover")
			sc.ProcAPI.Exited(proc.NewStatusErr("partitioned", nil))
			mv.exit = false // parent has received an exit status, so don't exit again
		}
	})

	checkFence(mv.FsLib, mv.job, mv.fence)

	return mv, nil
}

// Copy shard from src to dst
func (mv *Mover) moveShard(s, d string) (int, error) {
	if err := mv.kc.FreezeShard(s, mv.shard, mv.fence); err != nil {
		db.DPrintf(db.KVMV_ERR, "FreezeShard %v err %v\n", s, err)
		// did previous mover finish the job?
		if serr.IsErrCode(err, serr.TErrNotfound) {
			return 0, nil
		}
		return 0, err
	}

	vals, err := mv.kc.DumpShard(s, mv.shard, mv.fence)
	if err != nil {
		db.DPrintf(db.KVMV_ERR, "DumpShard %v err %v\n", mv.shard, err)
		return 0, err
	}

	if err := mv.kc.CreateShard(d, mv.shard, mv.fence, vals); err != nil {
		db.DPrintf(db.KVMV_ERR, "CreateShard %v err %v\n", mv.shard, err)
		return 0, err
	}

	// Mark that move is done by deleting s
	if err := mv.kc.DeleteShard(s, mv.shard, mv.fence); err != nil {
		db.DPrintf(db.KVMV_ERR, "DeleteShard src %v err %v\n", mv.shard, err)
		return 0, err
	}
	return len(vals), nil
}

type TmoverRes struct {
	Ms    int64 `json:"Ms"`
	Nkeys int64 `json:"Nkeys"`
}

func (mv *Mover) Move(src, dst string) {
	s := time.Now()
	db.DPrintf(db.KVMV, "conf %v: mov %v from %v to %v\n", mv.fence, mv.shard, src, dst)
	n, err := mv.moveShard(src, dst)
	if err != nil {
		db.DPrintf(db.KVMV_ERR, "conf %v: move %v from %v to %v err %v\n", mv.fence, mv.shard, src, dst, err)
	}
	db.DPrintf(db.KVMV, "conf %v: move %v done from %v to %v err %v\n", mv.fence, mv.shard, src, dst, err)
	if mv.exit {
		if err != nil {
			mv.ClntExit(proc.NewStatusErr(err.Error(), nil))
		} else {
			mv.ClntExit(proc.NewStatusInfo(proc.StatusOK, "OK", TmoverRes{time.Since(s).Milliseconds(), int64(n)}))
		}
	}
}
