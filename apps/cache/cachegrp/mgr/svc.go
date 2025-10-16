// The cachegrp package manages a service of cachesrvs.  Server i
// post itself with the pathname cachegrp.SRVDIR/i.
package mgr

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"

	"sigmaos/apps/cache"
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
	EPCacheJob       *epsrv.EPCacheJob
	epcsrvEP         *sp.Tendpoint
	useEPCache       bool
	bin              string
	servers          []sp.Tpid
	serverEPs        []*sp.Tendpoint
	backupServers    []sp.Tpid
	backupBootScript []byte
	scalerBootScript []byte
	cfg              *CacheJobConfig
	pn               string
	job              string
}

// Currently, only backup servers advertise themselves via the EP cache
func (cs *CachedSvc) addServer(i int) error {
	// SpawnBurst to spread servers across procds.
	p := proc.NewProc(cs.bin, []string{filepath.Join(cs.pn, cachegrp.SRVDIR), strconv.Itoa(int(i)), strconv.FormatBool(cs.useEPCache)})
	if !cs.cfg.GC {
		p.AppendEnv("GOGC", "off")
	}
	p.SetMcpu(cs.cfg.MCPU)
	err := cs.Spawn(p)
	if err != nil {
		return err
	}
	if err := cs.WaitStart(p.GetPid()); err != nil {
		return err
	}
	cs.servers = append(cs.servers, p.GetPid())
	if cs.useEPCache {
		// Get EP from epcachesrv
		pn := cs.Server(strconv.Itoa(i))
		svcName := filepath.Dir(pn)
		instances, _, err := cs.EPCacheJob.Clnt.GetEndpoints(svcName, epcache.NO_VERSION)
		if err != nil {
			db.DPrintf(db.CACHEDSVCCLNT, "Err get endpoints after adding cached server: %v", err)
			return err
		}
		istr := strconv.Itoa(i)
		var ep *sp.Tendpoint
		for _, is := range instances {
			if is.ID == istr {
				ep = sp.NewEndpointFromProto(is.EndpointProto)
			}
		}
		if ep == nil {
			db.DPrintf(db.ERROR, "Err get EP")
			return fmt.Errorf("Error get EP srv %v", i)
		}
		// Store the server EP for later use
		cs.serverEPs = append(cs.serverEPs, ep)
		// Manually mount cached so it will resolve later
		if err := cs.MountTree(ep, rpc.RPC, filepath.Join(pn, rpc.RPC)); err != nil {
			return err
		}
	}
	return nil
}

func (cs *CachedSvc) addBackupServerWithSigmaPath(sigmaPath string, srvID int, ep *sp.Tendpoint, delegatedInit bool, topN int) error {
	// SpawnBurst to spread servers across procds.
	p := proc.NewProc(cs.bin+"-backup", []string{filepath.Join(cs.pn, cachegrp.BACKUP), cs.job, strconv.Itoa(int(srvID)), strconv.FormatBool(cs.useEPCache), strconv.Itoa(topN)})
	if !cs.cfg.GC {
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
	p.SetMcpu(cs.cfg.MCPU)
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
		p.SetBootScript(cs.backupBootScript, bootScriptInput)
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
		svcName := filepath.Dir(backupPN)
		instances, _, err := cs.EPCacheJob.Clnt.GetEndpoints(svcName, epcache.NO_VERSION)
		if err != nil {
			return err
		}
		srvIDStr := strconv.Itoa(srvID)
		var ep *sp.Tendpoint
		for _, i := range instances {
			if i.ID == srvIDStr {
				ep = sp.NewEndpointFromProto(i.EndpointProto)
			}
		}
		if ep == nil {
			db.DPrintf(db.ERROR, "Err get EP")
			return fmt.Errorf("Error get EP srv %v", srvID)
		}
		// Manually mount cached-backup so it will resolve later
		if err := cs.MountTree(ep, rpc.RPC, filepath.Join(backupPN, rpc.RPC)); err != nil {
			return err
		}
	}
	cs.backupServers = append(cs.backupServers, p.GetPid())
	return nil
}

func (cs *CachedSvc) addScalerServerWithSigmaPath(sigmaPath string, delegatedInit bool, cpp bool, shmem bool) error {
	if shmem && !cpp {
		return fmt.Errorf("error shmem without cpp")
	}
	oldNSrv := len(cs.servers)
	newNSrv := oldNSrv + 1
	srvID := len(cs.servers)
	bin := cs.bin + "-scaler"
	if cpp {
		bin = "cached-srv-cpp"
	}
	p := proc.NewProc(bin, []string{filepath.Join(cs.pn, cachegrp.SRVDIR), cs.job, strconv.Itoa(srvID), strconv.FormatBool(cs.useEPCache), strconv.Itoa(oldNSrv), strconv.Itoa(newNSrv)})
	p.SetUseShmem(shmem)
	if !cs.cfg.GC {
		p.AppendEnv("GOGC", "off")
	}
	if sigmaPath != sp.NOT_SET {
		p.PrependSigmaPath(sigmaPath)
	}
	if cs.useEPCache {
		// Cache the primary server's endpoint in the scaler proc struct
		p.SetCachedEndpoint(epcache.EPCACHE, cs.epcsrvEP)
	}
	for i := 0; i < oldNSrv; i++ {
		// Cache the other cache servers' endpoint in the scaler proc struct
		p.SetCachedEndpoint(cs.Server(strconv.Itoa(i)), cs.serverEPs[i])
	}
	p.SetMcpu(cs.cfg.MCPU)
	// Have scaler server use spproxy
	p.GetProcEnv().UseSPProxy = true
	p.GetProcEnv().UseSPProxyProcClnt = true
	// Write the input arguments to the boot script
	inputBuf := bytes.NewBuffer(make([]byte, 0, 4))
	if err := binary.Write(inputBuf, binary.LittleEndian, uint32(srvID)); err != nil {
		return err
	}
	if err := binary.Write(inputBuf, binary.LittleEndian, uint32(oldNSrv)); err != nil {
		return err
	}
	if err := binary.Write(inputBuf, binary.LittleEndian, uint32(newNSrv)); err != nil {
		return err
	}
	if err := binary.Write(inputBuf, binary.LittleEndian, uint32(cache.NSHARD)); err != nil {
		return err
	}
	if delegatedInit {
		bootScriptInput := inputBuf.Bytes()
		p.SetBootScript(cs.scalerBootScript, bootScriptInput)
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
		srvPN := cs.Server(strconv.Itoa(srvID))
		svcName := filepath.Dir(srvPN)
		// Get EP from epcachesrv
		instances, _, err := cs.EPCacheJob.Clnt.GetEndpoints(svcName, epcache.NO_VERSION)
		if err != nil {
			return err
		}
		if len(instances) <= srvID {
			return fmt.Errorf("not enough instances reported by epcache: %v <= %v", len(instances), srvID)
		}
		srvIDStr := strconv.Itoa(srvID)
		var ep *sp.Tendpoint
		for _, i := range instances {
			if i.ID == srvIDStr {
				ep = sp.NewEndpointFromProto(i.EndpointProto)
			}
		}
		if ep == nil {
			db.DPrintf(db.ERROR, "Err get EP")
			return fmt.Errorf("Error get EP srv %v", srvID)
		}
		// Don't mount CPP cache servers (the clients will be created elsewhere, directly using the EP)
		if ep.GetType() != sp.CPP_EP {
			if err := cs.MountTree(ep, rpc.RPC, filepath.Join(srvPN, rpc.RPC)); err != nil {
				return err
			}
		}
		cs.serverEPs = append(cs.serverEPs, ep)
	}
	cs.servers = append(cs.servers, p.GetPid())
	return nil
}

// XXX use job
func NewCachedSvc(sc *sigmaclnt.SigmaClnt, cfg *CacheJobConfig, job, bin, pn string) (*CachedSvc, error) {
	return NewCachedSvcEPCache(sc, nil, cfg, job, bin, pn)
}

func NewCachedSvcEPCache(sc *sigmaclnt.SigmaClnt, epCacheJob *epsrv.EPCacheJob, cfg *CacheJobConfig, job, bin, pn string) (*CachedSvc, error) {
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
	backupBootScript, err := wasmer.ReadBootScript(sc, "cached_backup_hot_shard_boot")
	if err != nil {
		db.DPrintf(db.ERROR, "Err read WASM backup boot script: %v", err)
		return nil, err
	}
	scalerBootScript, err := wasmer.ReadBootScript(sc, "cached_scaler_boot")
	if err != nil {
		db.DPrintf(db.ERROR, "Err read WASM scaler boot script: %v", err)
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
		SigmaClnt:        sc,
		EPCacheJob:       epCacheJob,
		epcsrvEP:         epcsrvEP,
		useEPCache:       epCacheJob != nil,
		bin:              bin,
		servers:          make([]sp.Tpid, 0),
		serverEPs:        make([]*sp.Tendpoint, 0),
		backupServers:    make([]sp.Tpid, 0),
		cfg:              cfg,
		pn:               pn,
		job:              job,
		scalerBootScript: scalerBootScript,
		backupBootScript: backupBootScript,
	}
	for i := 0; i < cs.cfg.NSrv; i++ {
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

func (cs *CachedSvc) AddScalerServerWithSigmaPath(sigmaPath string, delegatedInit bool, cpp bool, shmem bool) error {
	cs.Lock()
	defer cs.Unlock()

	return cs.addScalerServerWithSigmaPath(sigmaPath, delegatedInit, cpp, shmem)
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
