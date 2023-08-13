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
	shards []sp.Tpid
	nshard int
	mcpu   proc.Tmcpu
	pn     string
	gc     bool
	public bool
}

func (sm *ShardMgr) addShard(i int) error {
	// SpawnBurst to spread shards across procds.
	p := proc.MakeProc(sm.bin, []string{sm.pn, strconv.FormatBool(sm.public), SHRDDIR + strconv.Itoa(int(i))})
	//	p.AppendEnv("GODEBUG", "gctrace=1")
	if !sm.gc {
		p.AppendEnv("GOGC", "off")
	}
	p.SetMcpu(sm.mcpu)
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

func MkShardMgr(sc *sigmaclnt.SigmaClnt, n int, mcpu proc.Tmcpu, job, bin, pn string, gc, public bool) (*ShardMgr, error) {
	sc.MkDir(pn, 0777)
	if _, err := sc.Create(pn+SHRDDIR, 0777|sp.DMDIR, sp.OREAD); err != nil {
		if !serr.IsErrCode(err, serr.TErrExists) {
			return nil, err
		}
	}
	sm := &ShardMgr{
		SigmaClnt: sc,
		bin:       bin,
		shards:    make([]sp.Tpid, 0),
		nshard:    n,
		mcpu:      mcpu,
		pn:        pn,
		gc:        gc,
		public:    public,
	}
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
