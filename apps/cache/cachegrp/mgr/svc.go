// The cachegrp package manages a service of cachesrvs.  Server i
// post itself with the pathname cachegrp.SRVDIR/i.
package mgr

import (
	"strconv"
	"sync"

	"sigmaos/apps/cache"
	"sigmaos/apps/cache/cachegrp"
	cacheproto "sigmaos/apps/cache/proto"
	db "sigmaos/debug"
	"sigmaos/proc"
	rpcclnt "sigmaos/rpc/clnt"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type CachedSvc struct {
	sync.Mutex
	*sigmaclnt.SigmaClnt
	bin           string
	servers       []sp.Tpid
	backupServers []sp.Tpid
	nserver       int
	mcpu          proc.Tmcpu
	pn            string
	job           string
	gc            bool
}

func (cs *CachedSvc) addServer(i int) error {
	// SpawnBurst to spread servers across procds.
	p := proc.NewProc(cs.bin, []string{cs.pn, cachegrp.SRVDIR + strconv.Itoa(int(i))})
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

func (cs *CachedSvc) addBackupServer(srvID int, delegatedInit bool) error {
	// SpawnBurst to spread servers across procds.
	p := proc.NewProc(cs.bin+"-backup", []string{cs.pn, cs.job, cachegrp.BACKUP + strconv.Itoa(int(srvID))})
	if !cs.gc {
		p.AppendEnv("GOGC", "off")
	}
	p.SetMcpu(cs.mcpu)
	// Have backup server use spproxy
	p.GetProcEnv().UseSPProxy = true
	p.GetProcEnv().UseSPProxyProcClnt = true

	totalInIOVLen := 0
	dumps := make([]*cacheproto.ShardReq, cache.NSHARD)
	for i := range dumps {
		dumps[i] = &cacheproto.ShardReq{
			Shard: uint32(i),
			Fence: sp.NullFence().FenceProto(),
		}
		cachesrvPN := cs.Server(strconv.Itoa(srvID))
		iniov, err := rpcclnt.WrapRPCRequest("CacheSrv.DumpShard", dumps[i])
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error wrap & marshal dumpReq: %v", err)
			return err
		}
		p.AddInitializationRPC(cachesrvPN, iniov, 2)
		for _, b := range iniov {
			totalInIOVLen += len(b)
		}
	}
	db.DPrintf(db.TEST, "Delegated RPC(%v) total len: %v", len(dumps), totalInIOVLen)
	// Ask for spproxy to run delegated initialization RPCs on behalf of the proc
	p.SetDelegateInit(delegatedInit)
	err := cs.Spawn(p)
	if err != nil {
		return err
	}
	if err := cs.WaitStart(p.GetPid()); err != nil {
		return err
	}
	cs.backupServers = append(cs.backupServers, p.GetPid())
	return nil
}

// XXX use job
func NewCachedSvc(sc *sigmaclnt.SigmaClnt, nsrv int, mcpu proc.Tmcpu, job, bin, pn string, gc bool) (*CachedSvc, error) {
	sc.MkDir(pn, 0777)
	if err := sc.MkDir(pn+cachegrp.SRVDIR, 0777); err != nil {
		if !serr.IsErrCode(err, serr.TErrExists) {
			return nil, err
		}
	}
	if err := sc.MkDir(pn+cachegrp.BACKUP, 0777); err != nil {
		if !serr.IsErrCode(err, serr.TErrExists) {
			return nil, err
		}
	}
	cs := &CachedSvc{
		SigmaClnt:     sc,
		bin:           bin,
		servers:       make([]sp.Tpid, 0),
		backupServers: make([]sp.Tpid, 0),
		nserver:       nsrv,
		mcpu:          mcpu,
		pn:            pn,
		gc:            gc,
		job:           job,
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

func (cs *CachedSvc) AddBackupServer(i int, delegatedInit bool) error {
	cs.Lock()
	defer cs.Unlock()

	return cs.addBackupServer(i, delegatedInit)
}

func (cs *CachedSvc) Nserver() int {
	return len(cs.servers)
}

func (cs *CachedSvc) SvcDir() string {
	return cs.pn
}

func (cs *CachedSvc) Server(n string) string {
	return cs.pn + cachegrp.Server(n)
}

func (cs *CachedSvc) BackupServer(n string) string {
	return cs.pn + cachegrp.BackupServer(n)
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
	for _, pid := range cs.backupServers {
		if err := cs.Evict(pid); err != nil {
			return err
		}
		if status, err := cs.WaitExit(pid); err != nil || !status.IsStatusOK() {
			return err
		}
	}
	cs.RmDir(cs.pn + cachegrp.SRVDIR)
	return nil
}
