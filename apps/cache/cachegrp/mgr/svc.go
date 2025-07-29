// The cachegrp package manages a service of cachesrvs.  Server i
// post itself with the pathname cachegrp.SRVDIR/i.
package mgr

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"strconv"
	"sync"

	"sigmaos/apps/cache/cachegrp"
	"sigmaos/apps/epcache"
	epsrv "sigmaos/apps/epcache/srv"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/proxy/wasm/rpc/wasmer"
	"sigmaos/rpc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type CachedSvc struct {
	sync.Mutex
	*sigmaclnt.SigmaClnt
	EPCacheJob    *epsrv.EPCacheJob
	epcsrvEP      *sp.Tendpoint
	useEPCache    bool
	bin           string
	servers       []sp.Tpid
	backupServers []sp.Tpid
	bootScript    []byte
	nserver       int
	mcpu          proc.Tmcpu
	pn            string
	job           string
	gc            bool
}

// Currently, only backup servers advertise themselves via the EP cache
func (cs *CachedSvc) addServer(i int) error {
	// SpawnBurst to spread servers across procds.
	p := proc.NewProc(cs.bin, []string{cs.pn, cachegrp.SRVDIR + strconv.Itoa(int(i)), strconv.FormatBool(false)})
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

func (cs *CachedSvc) addBackupServerWithSigmaPath(sigmaPath string, srvID int, ep *sp.Tendpoint, delegatedInit bool, topN int) error {
	// SpawnBurst to spread servers across procds.
	p := proc.NewProc(cs.bin+"-backup", []string{cs.pn, cs.job, cachegrp.BACKUP + strconv.Itoa(int(srvID)), strconv.FormatBool(cs.useEPCache), strconv.Itoa(topN)})
	if !cs.gc {
		p.AppendEnv("GOGC", "off")
	}
	if sigmaPath != sp.NOT_SET {
		p.PrependSigmaPath(sigmaPath)
	}
	if cs.useEPCache {
		// Cache the primary server's endpoint in the backup proc struct
		p.SetCachedEndpoint(epcache.EPCACHE, cs.epcsrvEP)
	}
	// Cache the primary server's endpoint in the backup proc struct
	p.SetCachedEndpoint(cs.Server(strconv.Itoa(srvID)), ep)
	p.SetMcpu(cs.mcpu)
	// Have backup server use spproxy
	p.GetProcEnv().UseSPProxy = true
	p.GetProcEnv().UseSPProxyProcClnt = true
	// Write the input arguments to the boot script
	inputBuf := bytes.NewBuffer(make([]byte, 0, 4))
	if err := binary.Write(inputBuf, binary.LittleEndian, uint32(srvID)); err != nil {
		return err
	}
	if err := binary.Write(inputBuf, binary.LittleEndian, uint32(topN)); err != nil {
		return err
	}
	if delegatedInit {
		bootScriptInput := inputBuf.Bytes()
		p.SetBootScript(cs.bootScript, bootScriptInput)
		p.SetRunBootScript(delegatedInit)
	}
	err := cs.Spawn(p)
	if err != nil {
		return err
	}
	if err := cs.WaitStart(p.GetPid()); err != nil {
		return err
	}
	if cs.useEPCache {
		backupPN := cs.BackupServer(strconv.Itoa(srvID))
		// Get EP from epcachesrv
		instances, _, err := cs.EPCacheJob.Clnt.GetEndpoints(backupPN, epcache.NO_VERSION)
		if err != nil {
			return err
		}
		// Manually mount cached-backup so it will resolve later
		ep := sp.NewEndpointFromProto(instances[0].EndpointProto)
		if err := cs.MountTree(ep, rpc.RPC, filepath.Join(backupPN, rpc.RPC)); err != nil {
			return err
		}
	}
	cs.backupServers = append(cs.backupServers, p.GetPid())
	return nil
}

// XXX use job
func NewCachedSvc(sc *sigmaclnt.SigmaClnt, nsrv int, mcpu proc.Tmcpu, job, bin, pn string, gc bool) (*CachedSvc, error) {
	return NewCachedSvcEPCache(sc, nil, nsrv, mcpu, job, bin, pn, gc)
}

func NewCachedSvcEPCache(sc *sigmaclnt.SigmaClnt, epCacheJob *epsrv.EPCacheJob, nsrv int, mcpu proc.Tmcpu, job, bin, pn string, gc bool) (*CachedSvc, error) {
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
	bootScript, err := wasmer.ReadBootScript(sc, "cached_backup_hot_shard_boot")
	if err != nil {
		db.DPrintf(db.ERROR, "Err read WASM boot script: %v", err)
		return nil, err
	}
	var epcsrvEP *sp.Tendpoint
	if epCacheJob != nil {
		// Cache the EPCache's endpoint in the proc stroct
		epcsrvEP, err = epCacheJob.GetSrvEP()
		if err != nil {
			db.DPrintf(db.ERROR, "Err getSrvEP ep cache srv: %v", err)
			return nil, err
		}
	}
	cs := &CachedSvc{
		SigmaClnt:     sc,
		EPCacheJob:    epCacheJob,
		epcsrvEP:      epcsrvEP,
		useEPCache:    epCacheJob != nil,
		bin:           bin,
		servers:       make([]sp.Tpid, 0),
		backupServers: make([]sp.Tpid, 0),
		nserver:       nsrv,
		mcpu:          mcpu,
		pn:            pn,
		gc:            gc,
		job:           job,
		bootScript:    bootScript,
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

func (cs *CachedSvc) AddBackupServerWithSigmaPath(sigmaPath string, i int, ep *sp.Tendpoint, delegatedInit bool, topN int) error {
	cs.Lock()
	defer cs.Unlock()

	return cs.addBackupServerWithSigmaPath(sigmaPath, i, ep, delegatedInit, topN)
}

func (cs *CachedSvc) AddBackupServer(i int, ep *sp.Tendpoint, delegatedInit bool, topN int) error {
	cs.Lock()
	defer cs.Unlock()

	return cs.AddBackupServerWithSigmaPath(sp.NOT_SET, i, ep, delegatedInit, topN)
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
