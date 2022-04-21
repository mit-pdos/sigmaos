package realm

import (
	"fmt"
	"log"
	"path"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"ulambda/config"
	"ulambda/ctx"
	db "ulambda/debug"
	"ulambda/dir"
	"ulambda/electclnt"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/kernel"
	np "ulambda/ninep"
	"ulambda/stats"
)

const (
	free_nodeds   = "free-nodeds"
	realm_create  = "realm-create"
	realm_destroy = "realm-destroy"
	FREE_NODEDS   = np.SIGMA_MGR + "/" + free_nodeds   // Unassigned nodeds
	REALM_CREATE  = np.SIGMA_MGR + "/" + realm_create  // Realm allocation requests
	REALM_DESTROY = np.SIGMA_MGR + "/" + realm_destroy // Realm destruction requests
	REALM_CONFIG  = "name/realm-config"                // Store of realm configs
	NODED_CONFIG  = "name/noded-config"                // Store of noded configs
	REALM_NAMEDS  = "name/realm-nameds"                // Symlinks to realms' nameds
	REALM_FENCES  = "name/realm-fences"                // Fence around modifications to realm allocations.
)

type SigmaResourceMgr struct {
	sync.Mutex
	freeNodeds   chan string
	realmCreate  chan string
	realmDestroy chan string
	root         fs.Dir
	ecs          map[string]*electclnt.ElectClnt
	*config.ConfigClnt
	*fslib.FsLib
	*fslibsrv.MemFs
}

func MakeSigmaResourceMgr() *SigmaResourceMgr {
	m := &SigmaResourceMgr{}
	m.freeNodeds = make(chan string)
	m.realmCreate = make(chan string)
	m.realmDestroy = make(chan string)
	var err error
	m.MemFs, m.FsLib, _, err = fslibsrv.MakeMemFs(np.SIGMA_MGR, "sigmamgr")
	if err != nil {
		db.DFatalf("Error MakeMemFs in MakeSigmaResourceMgr: %v", err)
	}
	m.ConfigClnt = config.MakeConfigClnt(m.FsLib)
	m.makeInitFs()
	m.makeCtlFiles()
	m.ecs = make(map[string]*electclnt.ElectClnt)

	return m
}

// Make the initial realm dirs, and remove the unneeded union dirs.
func (m *SigmaResourceMgr) makeInitFs() {
	if err := m.MkDir(REALM_CONFIG, 0777); err != nil {
		db.DFatalf("Error Mkdir REALM_CONFIG in SigmaResourceMgr.makeInitFs: %v", err)
	}
	if err := m.MkDir(NODED_CONFIG, 0777); err != nil {
		db.DFatalf("Error Mkdir NODED_CONFIG in SigmaResourceMgr.makeInitFs: %v", err)
	}
	if err := m.MkDir(REALM_NAMEDS, 0777); err != nil {
		db.DFatalf("Error Mkdir REALM_NAMEDS in SigmaResourceMgr.makeInitFs: %v", err)
	}
	if err := m.MkDir(REALM_FENCES, 0777); err != nil {
		db.DFatalf("Error Mkdir REALM_FENCES in SigmaResourceMgr.makeInitFs: %v", err)
	}
}

func (m *SigmaResourceMgr) makeCtlFiles() {
	// Set up control files
	realmCreate := makeCtlFile(m.realmCreate, nil, m.Root())
	err := dir.MkNod(ctx.MkCtx("", 0, nil), m.Root(), realm_create, realmCreate)
	if err != nil {
		db.DFatalf("Error MkNod in SigmaResourceMgr.makeCtlFiles 1: %v", err)
	}

	realmDestroy := makeCtlFile(m.realmDestroy, nil, m.Root())
	err = dir.MkNod(ctx.MkCtx("", 0, nil), m.Root(), realm_destroy, realmDestroy)
	if err != nil {
		db.DFatalf("Error MkNod in SigmaResourceMgr.makeCtlFiles 2: %v", err)
	}

	freeNodeds := makeCtlFile(m.freeNodeds, nil, m.Root())
	err = dir.MkNod(ctx.MkCtx("", 0, nil), m.Root(), free_nodeds, freeNodeds)
	if err != nil {
		db.DFatalf("Error MkNod in SigmaResourceMgr.makeCtlFiles 3: %v", err)
	}
}

func (m *SigmaResourceMgr) lockRealm(realmId string) {
	if err := m.ecs[realmId].AcquireLeadership([]byte("sigmamgr")); err != nil {
		db.DFatalf("%v error SigmaResourceMgr acquire leadership: %v", string(debug.Stack()), err)
	}
}

func (m *SigmaResourceMgr) unlockRealm(realmId string) {
	if err := m.ecs[realmId].ReleaseLeadership(); err != nil {
		db.DFatalf("%v error SigmaResourceMgr release leadership: %v", string(debug.Stack()), err)
	}
}

// Handle realm creation requests.
func (m *SigmaResourceMgr) createRealms() {
	for {
		// Get a realm creation request
		realmId := <-m.realmCreate

		m.Lock()
		// Make sure we haven't created this realm before.
		if _, ok := m.ecs[realmId]; ok {
			db.DFatalf("tried to create realm twice %v", realmId)
		}
		m.ecs[realmId] = electclnt.MakeElectClnt(m.FsLib, path.Join(REALM_FENCES, realmId), 0777)

		m.lockRealm(realmId)

		cfg := &RealmConfig{}
		cfg.Rid = realmId

		// Make the realm config file.
		m.WriteConfig(path.Join(REALM_CONFIG, realmId), cfg)

		m.unlockRealm(realmId)
		m.Unlock()
	}
}

// Deallocate a noded from a realm.
func (m *SigmaResourceMgr) deallocNoded(realmId string, nodedId string) {
	rdCfg := &NodedConfig{}
	rdCfg.Id = nodedId
	rdCfg.RealmId = kernel.NO_REALM

	// Update the noded config file.
	m.WriteConfig(path.Join(NODED_CONFIG, nodedId), rdCfg)

	// Note noded de-registration
	rCfg := &RealmConfig{}
	m.ReadConfig(path.Join(REALM_CONFIG, realmId), rCfg)
	// Remove the noded from the lsit of assigned nodeds.
	for i := range rCfg.NodedsAssigned {
		if rCfg.NodedsAssigned[i] == nodedId {
			rCfg.NodedsAssigned = append(rCfg.NodedsAssigned[:i], rCfg.NodedsAssigned[i+1:]...)
		}
	}
	rCfg.LastResize = time.Now()
	m.WriteConfig(path.Join(REALM_CONFIG, realmId), rCfg)
}

func (m *SigmaResourceMgr) deallocAllNodeds(realmId string, nodedIds []string) {
	for _, nodedId := range nodedIds {
		m.deallocNoded(realmId, nodedId)
	}
}

func (m *SigmaResourceMgr) destroyRealms() {
	for {
		// Get a realm creation request
		realmId := <-m.realmDestroy

		m.Lock()
		m.lockRealm(realmId)

		cfg := &RealmConfig{}
		m.ReadConfig(path.Join(REALM_CONFIG, realmId), cfg)

		m.deallocAllNodeds(realmId, cfg.NodedsAssigned)

		cfg.Shutdown = true
		m.WriteConfig(path.Join(REALM_CONFIG, realmId), cfg)

		m.unlockRealm(realmId)
		delete(m.ecs, realmId)
		m.Unlock()
	}
}

// Get & alloc a noded to this realm. Return true if successful
func (m *SigmaResourceMgr) allocNoded(realmId string) bool {
	// Get a free noded
	select {
	// If there is a noded available...
	case nodedId := <-m.freeNodeds:
		// Update the noded's config
		rdCfg := &NodedConfig{}
		rdCfg.Id = nodedId
		rdCfg.RealmId = realmId
		m.WriteConfig(path.Join(NODED_CONFIG, nodedId), rdCfg)

		// Update the realm's config
		rCfg := &RealmConfig{}
		m.ReadConfig(path.Join(REALM_CONFIG, realmId), rCfg)
		rCfg.NodedsAssigned = append(rCfg.NodedsAssigned, nodedId)
		rCfg.LastResize = time.Now()
		m.WriteConfig(path.Join(REALM_CONFIG, realmId), rCfg)
		return true
	default:
		// If no noded is available...
		return false
	}
}

// Get all a realm's procd's stats
func (m *SigmaResourceMgr) getRealmProcdStats(nameds []string, realmId string) map[string]*stats.StatInfo {
	// XXX For now we assume all the nameds are up
	stat := make(map[string]*stats.StatInfo)
	if len(nameds) == 0 {
		return stat
	}
	// XXX May fail if this named crashed
	procds, err := m.GetDir(path.Join(REALM_NAMEDS, realmId, np.PROCDREL))
	if err != nil {
		db.DFatalf("Error GetDir 2 in SigmaResourceMgr.getRealmProcdStats: %v", err)
	}
	for _, pd := range procds {
		s := &stats.StatInfo{}
		// XXX May fail if this named crashed
		err := m.GetFileJson(path.Join(REALM_NAMEDS, realmId, np.PROCDREL, pd.Name, np.STATSD), s)
		if err != nil {
			log.Printf("Error ReadFileJson in SigmaResourceMgr.getRealmProcdStats: %v", err)
			continue
		}
		stat[pd.Name] = s
	}
	return stat
}

func (m *SigmaResourceMgr) getRealmConfig(realmId string) (*RealmConfig, error) {
	// If the realm is being shut down, the realm config file may not be there
	// anymore. In this case, another noded is not needed.
	if _, err := m.Stat(path.Join(REALM_CONFIG, realmId)); err != nil && strings.Contains(err.Error(), "file not found") {
		return nil, fmt.Errorf("Realm not found")
	}
	cfg := &RealmConfig{}
	m.ReadConfig(path.Join(REALM_CONFIG, realmId), cfg)
	return cfg, nil
}

func (m *SigmaResourceMgr) getRealmUtil(realmId string, cfg *RealmConfig) (float64, map[string]float64) {
	// Get stats
	utilMap := make(map[string]float64)
	procdStats := m.getRealmProcdStats(cfg.NamedAddrs, realmId)
	avgUtil := 0.0
	for nodedId, stat := range procdStats {
		avgUtil += stat.Util
		utilMap[nodedId] = stat.Util
	}
	if len(procdStats) > 0 {
		avgUtil /= float64(len(procdStats))
	}
	return avgUtil, utilMap
}

func (m *SigmaResourceMgr) adjustRealm(realmId string) {
	// Get the realm's config
	realmCfg, err := m.getRealmConfig(realmId)
	if err != nil {
		db.DPrintf("SIGMAMGR", "Error SigmaResourceMgr.getRealmConfig in SigmaResourceMgr.adjustRealm: %v", err)
		return
	}

	// If the realm is shutting down, return
	if realmCfg.Shutdown {
		return
	}

	// If we are below the target replication level
	if len(realmCfg.NodedsAssigned) < nReplicas() {
		// Start enough nodeds to reach the target replication level
		for i := len(realmCfg.NodedsAssigned); i < nReplicas(); i++ {
			if ok := m.allocNoded(realmId); !ok {
				log.Printf("Error in adjustRealm: not enough nodeds to meet minimum replication level for realm %v", realmId)
			}
		}
		return
	}

	// If we have resized too recently, return
	if time.Now().Sub(realmCfg.LastResize).Milliseconds() < np.REALM_RESIZE_INTERVAL_MS {
		return
	}

	//	log.Printf("Avg util pre: %v", realmCfg)
	avgUtil, procdUtils := m.getRealmUtil(realmId, realmCfg)
	//	log.Printf("Avg util post: %v, %v", realmCfg, avgUtil)
	if avgUtil > np.REALM_GROW_CPU_UTIL_THRESHOLD {
		m.allocNoded(realmId)
	} else if avgUtil < np.REALM_SHRINK_CPU_UTIL_THRESHOLD {
		// If there are replicas to spare
		if len(realmCfg.NodedsAssigned) > nReplicas() {
			// Find least utilized procd
			//			min := 100.0
			//			minNodedId := ""
			//			for nodedId, util := range procdUtils {
			//				if min > util {
			//					min = util
			//					minNodedId = nodedId
			//				}
			//			}
			// XXX A hack for now, since we don't have a good way of linking a procd to a noded
			_ = procdUtils
			minNodedId := realmCfg.NodedsAssigned[1]
			// Deallocate least utilized procd
			m.deallocNoded(realmId, minNodedId)
		}
	}
}

// Balance nodeds across realms.
func (m *SigmaResourceMgr) balanceNodeds() {
	for {
		realms, err := m.GetDir(REALM_CONFIG)
		if err != nil {
			db.DFatalf("Error GetDir in SigmaResourceMgr.balanceNodeds: %v", err)
		}

		m.Lock()

		for _, realm := range realms {
			realmId := realm.Name
			// Realm must have exited
			if _, ok := m.ecs[realmId]; !ok {
				continue
			}
			m.lockRealm(realmId)

			// XXX Currently we assume there are always enough nodeds for the number
			// of realms we have. If that assumption is broken, this may deadlock when
			// a realm is trying to exit & we're trying to assign a noded to it.
			m.adjustRealm(realmId)
			m.unlockRealm(realmId)
		}

		m.Unlock()

		time.Sleep(np.REALM_SCAN_INTERVAL_MS * time.Millisecond)
	}
}

func (m *SigmaResourceMgr) Work() {
	go m.createRealms()
	go m.destroyRealms()
	go m.balanceNodeds()
	m.Serve()
	m.Done()
}
