// The cachedsvc package manages a service of cachesrvs.  Server i
// post itself with the pathname SRVDIR/i.
package cachedsvc

import (
	"strconv"
	"sync"

	// db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	SRVDIR = "servers/"
)

type CachedSvc struct {
	sync.Mutex
	*sigmaclnt.SigmaClnt
	bin     string
	servers []sp.Tpid
	nserver int
	mcpu    proc.Tmcpu
	pn      string
	gc      bool
}

func (cs *CachedSvc) addServer(i int) error {
	// SpawnBurst to spread servers across procds.
	p := proc.NewProc(cs.bin, []string{cs.pn, SRVDIR + strconv.Itoa(int(i))})
	//	p.AppendEnv("GODEBUG", "gctrace=1")
	if !cs.gc {
		p.AppendEnv("GOGC", "off")
	}
	p.SetMcpu(cs.mcpu)
	err := cs.Spawn(p)
	if err != nil {
		return err
	}
	if err := cs.WaitStart(p.GetPid()); err != nil {
		return err
	}
	cs.servers = append(cs.servers, p.GetPid())
	return nil
}

// XXX use job
func NewCachedSvc(sc *sigmaclnt.SigmaClnt, nsrv int, mcpu proc.Tmcpu, job, bin, pn string, gc bool) (*CachedSvc, error) {
	sc.MkDir(pn, 0777)
	if err := sc.MkDir(pn+SRVDIR, 0777); err != nil {
		if !serr.IsErrCode(err, serr.TErrExists) {
			return nil, err
		}
	}
	cs := &CachedSvc{
		SigmaClnt: sc,
		bin:       bin,
		servers:   make([]sp.Tpid, 0),
		nserver:   nsrv,
		mcpu:      mcpu,
		pn:        pn,
		gc:        gc,
	}
	for i := 0; i < cs.nserver; i++ {
		if err := cs.addServer(i); err != nil {
			return nil, err
		}
	}
	return cs, nil
}

func (cs *CachedSvc) AddServer() error {
	cs.Lock()
	defer cs.Unlock()

	n := len(cs.servers)
	return cs.addServer(n)
}

func Server(n string) string {
	return SRVDIR + n
}

func (cs *CachedSvc) Nserver() int {
	return len(cs.servers)
}

func (cs *CachedSvc) SvcDir() string {
	return cs.pn
}

func (cs *CachedSvc) Server(n string) string {
	return cs.pn + Server(n)
}

func (cs *CachedSvc) Stop() error {
	for _, pid := range cs.servers {
		if err := cs.Evict(pid); err != nil {
			return err
		}
		if _, err := cs.WaitExit(pid); err != nil {
			return err
		}
	}
	cs.RmDir(cs.pn + SRVDIR)
	return nil
}
