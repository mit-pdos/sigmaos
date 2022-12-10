package shardsvcmgr

import (
	"strconv"

	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procclnt"
)

// XXX shard should be a proc or a group

type ShardMgr struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	bin    string
	job    string
	shards []proc.Tpid
	nshard int
	pn     string
}

func MkShardMgr(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, n int, job, bin, pn string) (*ShardMgr, error) {
	sm := &ShardMgr{fsl, pclnt, bin, job, make([]proc.Tpid, n), n, pn}
	for i := 0; i < n; i++ {
		p := proc.MakeProc(bin, []string{job, strconv.Itoa(i)})
		if err := pclnt.Spawn(p); err != nil {
			return nil, err
		}
		if err := pclnt.WaitStart(p.Pid); err != nil {
			return nil, err
		}
		sm.shards[i] = p.Pid
	}
	return sm, nil
}

func Shard(i int) string {
	return strconv.Itoa(i)
}

func (sm *ShardMgr) Server(i int) string {
	return sm.pn + Shard(i)
}

func (sm *ShardMgr) Stop() error {
	for _, pid := range sm.shards {
		if err := sm.Evict(pid); err != nil {
			return err
		}
		if _, err := sm.WaitExit(pid); err != nil {
			return err
		}
	}
	return nil
}
