package shardsvcmgr

import (
	"strconv"
	"sync"

	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	SVRDIR = "servers/"
)

type ServerMgr struct {
	sync.Mutex
	*sigmaclnt.SigmaClnt
	bin     string
	servers []proc.Tpid
	nserver int
	mcpu    proc.Tmcpu
	pn      string
	gc      bool
	public  bool
}

func (sm *ServerMgr) addServer(i int) error {
	// SpawnBurst to spread servers across procds.
	p := proc.MakeProc(sm.bin, []string{sm.pn, strconv.FormatBool(sm.public), SVRDIR + strconv.Itoa(int(i))})
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
	sm.servers = append(sm.servers, p.GetPid())
	return nil
}

func MkServerMgr(sc *sigmaclnt.SigmaClnt, nsrv int, mcpu proc.Tmcpu, job, bin, pn string, gc, public bool) (*ServerMgr, error) {
	sc.MkDir(pn, 0777)
	if _, err := sc.Create(pn+SVRDIR, 0777|sp.DMDIR, sp.OREAD); err != nil {
		if !serr.IsErrCode(err, serr.TErrExists) {
			return nil, err
		}
	}
	sm := &ServerMgr{
		SigmaClnt: sc,
		bin:       bin,
		servers:   make([]proc.Tpid, 0),
		nserver:   nsrv,
		mcpu:      mcpu,
		pn:        pn,
		gc:        gc,
		public:    public,
	}
	for i := 0; i < sm.nserver; i++ {
		if err := sm.addServer(i); err != nil {
			return nil, err
		}
	}
	return sm, nil
}

func (sm *ServerMgr) AddServer() error {
	sm.Lock()
	defer sm.Unlock()

	n := len(sm.servers)
	return sm.addServer(n)
}

func Server(i int) string {
	return SVRDIR + strconv.Itoa(i)
}

func (sm *ServerMgr) Nserver() int {
	return len(sm.servers)
}

func (sm *ServerMgr) SvcDir() string {
	return sm.pn
}

func (sm *ServerMgr) Server(i int) string {
	return sm.pn + Server(i)
}

func (sm *ServerMgr) Stop() error {
	for _, pid := range sm.servers {
		if err := sm.Evict(pid); err != nil {
			return err
		}
		if _, err := sm.WaitExit(pid); err != nil {
			return err
		}
	}
	return nil
}
