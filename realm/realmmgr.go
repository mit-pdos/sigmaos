package realm

import (
	"fmt"
	"path"
	"strings"
	"time"

	"ulambda/config"
	"ulambda/ctx"
	db "ulambda/debug"
	"ulambda/dir"
	"ulambda/electclnt"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/kernel"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/stats"
)

const (
	REALMMGR_ELECT = "-realmmgr-elect"
)

type RealmResourceMgr struct {
	realmId string
	// ===== Relative to the sigma named =====
	sigmaFsl *fslib.FsLib
	ec       *electclnt.ElectClnt
	*config.ConfigClnt
	// ===== Relative to the realm named =====
	*procclnt.ProcClnt
	*fslib.FsLib
	*fslibsrv.MemFs
}

// XXX Make this proc un-stealable
func MakeRealmResourceMgr(rid string, sigmaNamedAddrs []string) *RealmResourceMgr {
	db.DPrintf("REALM", "MakeRealmResourceMgr")
	m := &RealmResourceMgr{}
	m.realmId = rid
	m.sigmaFsl = fslib.MakeFsLibAddr("realmmgr-sigmafsl", sigmaNamedAddrs)
	m.ConfigClnt = config.MakeConfigClnt(m.sigmaFsl)
	m.ec = electclnt.MakeElectClnt(m.sigmaFsl, path.Join(REALM_FENCES, rid+REALMMGR_ELECT), 0777)

	// XXX Currently, because of this scheme in which each noded runs a realmmgr,
	// the sigmamgr and other requesting applications may have to retry on
	// TErrNotfound.

	// Acquire primary role
	if err := m.ec.AcquireLeadership([]byte("realmmgr")); err != nil {
		db.DFatalf("Acquire leadership: %v", err)
	}

	var err error
	m.MemFs, m.FsLib, m.ProcClnt, err = fslibsrv.MakeMemFs(np.REALM_MGR, "realmmgr")
	if err != nil {
		db.DFatalf("Error MakeMemFs in MakeSigmaResourceMgr: %v", err)
	}

	m.makeCtlFiles()

	return m
}

func (m *RealmResourceMgr) makeCtlFiles() {
	// Set up control files
	ctl := makeCtlFile(m.receiveResourceGrant, m.handleResourceRequest, nil, m.Root())
	err := dir.MkNod(ctx.MkCtx("", 0, nil), m.Root(), sigmactl, ctl)
	if err != nil {
		db.DFatalf("Error MkNod sigmactl: %v", err)
	}
}

func (m *RealmResourceMgr) receiveResourceGrant(msg *ResourceMsg) {
	switch msg.ResourceType {
	case Tnode:
		db.DPrintf(db.ALWAYS, "Tnode granted")
		// TODO: implement
	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

func (m *RealmResourceMgr) handleResourceRequest(msg *ResourceMsg) {
	switch msg.ResourceType {
	case Tnode:
		db.DPrintf(db.ALWAYS, "Tnode requested")
		// TODO: implement
	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

// Deallocate a noded from a realm.
func (m *RealmResourceMgr) deallocNoded(nodedId string) {
	rdCfg := &NodedConfig{}
	rdCfg.Id = nodedId
	rdCfg.RealmId = kernel.NO_REALM

	// Update the noded config file.
	m.WriteConfig(path.Join(NODED_CONFIG, nodedId), rdCfg)

	// Note noded de-registration
	rCfg := &RealmConfig{}
	m.ReadConfig(path.Join(REALM_CONFIG, m.realmId), rCfg)
	// Remove the noded from the lsit of assigned nodeds.
	for i := range rCfg.NodedsAssigned {
		if rCfg.NodedsAssigned[i] == nodedId {
			rCfg.NodedsAssigned = append(rCfg.NodedsAssigned[:i], rCfg.NodedsAssigned[i+1:]...)
		}
	}
	rCfg.LastResize = time.Now()
	m.WriteConfig(path.Join(REALM_CONFIG, m.realmId), rCfg)
}

func (m *RealmResourceMgr) getRealmConfig(realmId string) (*RealmConfig, error) {
	// If the realm is being shut down, the realm config file may not be there
	// anymore. In this case, another noded is not needed.
	if _, err := m.sigmaFsl.Stat(path.Join(REALM_CONFIG, realmId)); err != nil && strings.Contains(err.Error(), "file not found") {
		return nil, fmt.Errorf("Realm not found")
	}
	cfg := &RealmConfig{}
	m.ReadConfig(path.Join(REALM_CONFIG, realmId), cfg)
	return cfg, nil
}

// Get all a realm's procd's stats
func (m *RealmResourceMgr) getRealmProcdStats(nameds []string, realmId string) map[string]*stats.StatInfo {
	// XXX For now we assume all the nameds are up
	stat := make(map[string]*stats.StatInfo)
	if len(nameds) == 0 {
		return stat
	}
	m.ProcessDir(np.KPIDS, func(st *np.Stat) (bool, error) {
		// If this is a procd...
		if strings.HasPrefix(st.Name, np.PROCDREL) {
			si := kernel.GetSubsystemInfo(m.FsLib, proc.Tpid(st.Name))
			s := &stats.StatInfo{}
			err := m.GetFileJson(path.Join(np.PROCD, si.Ip, np.STATSD), s)
			if err != nil {
				db.DPrintf(db.ALWAYS, "Error ReadFileJson in SigmaResourceMgr.getRealmProcdStats: %v", err)
				return false, nil
			}
			stat[si.NodedId] = s
		}
		return false, nil
	})
	return stat
}

func (m *RealmResourceMgr) getRealmUtil(realmId string, cfg *RealmConfig) (float64, map[string]float64) {
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

func (m *RealmResourceMgr) adjustRealm(realmId string) {
	// Get the realm's config
	realmCfg, err := m.getRealmConfig(realmId)
	if err != nil {
		db.DPrintf("REALMMGR", "Error getRealmConfig: %v", err)
		return
	}

	// If the realm is shutting down, return
	if realmCfg.Shutdown {
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
		// TODO:
		//		m.allocNoded(realmId)
	} else if avgUtil < np.REALM_SHRINK_CPU_UTIL_THRESHOLD {
		// If there are replicas to spare
		if len(realmCfg.NodedsAssigned) > nReplicas() {
			// Find least utilized procd
			min := 100.0
			minNodedId := ""
			for nodedId, util := range procdUtils {
				if min > util {
					min = util
					minNodedId = nodedId
				}
			}
			// Deallocate least utilized procd
			m.deallocNoded(minNodedId)
		}
	}
}

func (m *RealmResourceMgr) run() {
	// TODO:
	// * Acquire leadership
	// * Scan procds to calculate utilization.
	// * If utilization is high, request resources from sigma.
	// * If utilization is low, return resources to sigma.
}
