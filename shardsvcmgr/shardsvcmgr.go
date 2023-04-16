package shardsvcmgr

import (
	"strconv"
	"sync"

	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

// XXX shard should be a proc or a group

const (
	SHRDDIR = "shard/"
)

type ShardMgr struct {
	sync.Mutex
	*sigmaclnt.SigmaClnt
	bin    string
	job    string
	shards []proc.Tpid
	nshard int
	ncore  proc.Tcore
	pn     string
	public bool
}

func (sm *ShardMgr) addShard(i int) error {
	// SpawnBurst to spread shards across procds.
	p := proc.MakeProc(sm.bin, []string{sm.job, strconv.FormatBool(sm.public), SHRDDIR + strconv.Itoa(i)})
	//	p.AppendEnv("GODEBUG", "gctrace=1")
	p.AppendEnv("GOGC", "off")
	p.SetNcore(sm.ncore)
	_, errs := sm.SpawnBurst([]*proc.Proc{p}, 2)
	if len(errs) > 0 {
		return errs[0]
	}
	if err := sm.WaitStart(p.GetPid()); err != nil {
		return err
	}
	sm.shards = append(sm.shards, p.GetPid())
	return nil
}

func MkShardMgr(sc *sigmaclnt.SigmaClnt, n int, ncore proc.Tcore, job, bin, pn string, public bool) (*ShardMgr, error) {
	if _, err := sc.Create(pn+SHRDDIR, 0777|sp.DMDIR, sp.OREAD); err != nil {
		if !serr.IsErrCode(err, serr.TErrExists) {
			return nil, err
		}
	}
	sm := &ShardMgr{SigmaClnt: sc, bin: bin, job: job, shards: make([]proc.Tpid, 0), nshard: n, ncore: ncore, pn: pn, public: public}
	for i := 0; i < n; i++ {
		if err := sm.addShard(i); err != nil {
			return nil, err
		}
	}
	return sm, nil
}

func (sm *ShardMgr) AddShard() error {
	sm.Lock()
	defer sm.Unlock()

	n := len(sm.shards)
	return sm.addShard(n)
}

func Shard(i int) string {
	return SHRDDIR + strconv.Itoa(i)
}

func (sm *ShardMgr) Nshard() int {
	return len(sm.shards)
}

func (sm *ShardMgr) SvcDir() string {
	return sm.pn
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
