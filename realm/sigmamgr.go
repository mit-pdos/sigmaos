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
	free_machineds  = "free-machineds"
	realm_create    = "realm-create"
	realm_destroy   = "realm-destroy"
	FREE_MACHINEDS  = np.SIGMA_MGR + "/" + free_machineds // Unassigned machineds
	REALM_CREATE    = np.SIGMA_MGR + "/" + realm_create   // Realm allocation requests
	REALM_DESTROY   = np.SIGMA_MGR + "/" + realm_destroy  // Realm destruction requests
	REALM_CONFIG    = "name/realm-config"                 // Store of realm configs
	MACHINED_CONFIG = "name/machined-config"              // Store of machined configs
	REALM_NAMEDS    = "name/realm-nameds"                 // Symlinks to realms' nameds
	REALM_FENCES    = "name/realm-fences"                 // Fence around modifications to realm allocations.
)

type SigmaResourceMgr struct {
	sync.Mutex
	freeMachineds chan string
	realmCreate   chan string
	realmDestroy  chan string
	root          fs.Dir
	ecs           map[string]*electclnt.ElectClnt
	*config.ConfigClnt
	*fslib.FsLib
	*fslibsrv.MemFs
}

func MakeSigmaResourceMgr() *SigmaResourceMgr {
	m := &SigmaResourceMgr{}
	m.freeMachineds = make(chan string)
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
	if err := m.MkDir(MACHINED_CONFIG, 0777); err != nil {
		db.DFatalf("Error Mkdir MACHINED_CONFIG in SigmaResourceMgr.makeInitFs: %v", err)
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

	freeMachineds := makeCtlFile(m.freeMachineds, nil, m.Root())
	err = dir.MkNod(ctx.MkCtx("", 0, nil), m.Root(), free_machineds, freeMachineds)
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

// Deallocate a machined from a realm.
func (m *SigmaResourceMgr) deallocMachined(realmId string, machinedId string) {
	rdCfg := &MachinedConfig{}
	rdCfg.Id = machinedId
	rdCfg.RealmId = kernel.NO_REALM

	// Update the machined config file.
	m.WriteConfig(path.Join(MACHINED_CONFIG, machinedId), rdCfg)

	// Note machined de-registration
	rCfg := &RealmConfig{}
	m.ReadConfig(path.Join(REALM_CONFIG, realmId), rCfg)
	// Remove the machined from the lsit of assigned machineds.
	for i := range rCfg.MachinedsAssigned {
		if rCfg.MachinedsAssigned[i] == machinedId {
			rCfg.MachinedsAssigned = append(rCfg.MachinedsAssigned[:i], rCfg.MachinedsAssigned[i+1:]...)
		}
	}
	rCfg.LastResize = time.Now()
	m.WriteConfig(path.Join(REALM_CONFIG, realmId), rCfg)
}

func (m *SigmaResourceMgr) deallocAllMachineds(realmId string, machinedIds []string) {
	for _, machinedId := range machinedIds {
		m.deallocMachined(realmId, machinedId)
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

		m.deallocAllMachineds(realmId, cfg.MachinedsAssigned)

		cfg.Shutdown = true
		m.WriteConfig(path.Join(REALM_CONFIG, realmId), cfg)

		m.unlockRealm(realmId)
		delete(m.ecs, realmId)
		m.Unlock()
	}
}

// Get & alloc a machined to this realm. Return true if successful
func (m *SigmaResourceMgr) allocMachined(realmId string) bool {
	// Get a free machined
	select {
	// If there is a machined available...
	case machinedId := <-m.freeMachineds:
		// Update the machined's config
		rdCfg := &MachinedConfig{}
		rdCfg.Id = machinedId
		rdCfg.RealmId = realmId
		m.WriteConfig(path.Join(MACHINED_CONFIG, machinedId), rdCfg)

		// Update the realm's config
		rCfg := &RealmConfig{}
		m.ReadConfig(path.Join(REALM_CONFIG, realmId), rCfg)
		rCfg.MachinedsAssigned = append(rCfg.MachinedsAssigned, machinedId)
		rCfg.LastResize = time.Now()
		m.WriteConfig(path.Join(REALM_CONFIG, realmId), rCfg)
		return true
	default:
		// If no machined is available...
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
	// anymore. In this case, another machined is not needed.
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
	for machinedId, stat := range procdStats {
		avgUtil += stat.Util
		utilMap[machinedId] = stat.Util
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
	if len(realmCfg.MachinedsAssigned) < nReplicas() {
		// Start enough machineds to reach the target replication level
		for i := len(realmCfg.MachinedsAssigned); i < nReplicas(); i++ {
			if ok := m.allocMachined(realmId); !ok {
				log.Printf("Error in adjustRealm: not enough machineds to meet minimum replication level for realm %v", realmId)
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
		m.allocMachined(realmId)
	} else if avgUtil < np.REALM_SHRINK_CPU_UTIL_THRESHOLD {
		// If there are replicas to spare
		if len(realmCfg.MachinedsAssigned) > nReplicas() {
			// Find least utilized procd
			//			min := 100.0
			//			minMachinedId := ""
			//			for machinedId, util := range procdUtils {
			//				if min > util {
			//					min = util
			//					minMachinedId = machinedId
			//				}
			//			}
			// XXX A hack for now, since we don't have a good way of linking a procd to a machined
			_ = procdUtils
			minMachinedId := realmCfg.MachinedsAssigned[1]
			// Deallocate least utilized procd
			m.deallocMachined(realmId, minMachinedId)
		}
	}
}

// Balance machineds across realms.
func (m *SigmaResourceMgr) balanceMachineds() {
	for {
		realms, err := m.GetDir(REALM_CONFIG)
		if err != nil {
			db.DFatalf("Error GetDir in SigmaResourceMgr.balanceMachineds: %v", err)
		}

		m.Lock()

		for _, realm := range realms {
			realmId := realm.Name
			// Realm must have exited
			if _, ok := m.ecs[realmId]; !ok {
				continue
			}
			m.lockRealm(realmId)

			// XXX Currently we assume there are always enough machineds for the number
			// of realms we have. If that assumption is broken, this may deadlock when
			// a realm is trying to exit & we're trying to assign a machined to it.
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
	go m.balanceMachineds()
	m.Serve()
	m.Done()
}
