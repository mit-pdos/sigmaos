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

/* RealmMgr responsibilities:
 * - Respond to resource requests from SigmaMgr, and deallocate Nodeds.
 * - Ask for more Nodeds from SigmaMgr when load increases.
 */

const (
	REALMMGR_ELECT = "-realmmgr-elect"
	realmctl       = "realmctl"
)

type RealmResourceMgr struct {
	realmId string
	// ===== Relative to the sigma named =====
	sigmaFsl *fslib.FsLib
	ec       *electclnt.ElectClnt
	lock     *electclnt.ElectClnt
	*config.ConfigClnt
	// ===== Relative to the realm named =====
	*procclnt.ProcClnt
	*fslib.FsLib
	*fslibsrv.MemFs
}

// TODO: Make this proc un-stealable
func MakeRealmResourceMgr(rid string, sigmaNamedAddrs []string) *RealmResourceMgr {
	db.DPrintf("REALMMGR", "MakeRealmResourceMgr")
	db.DPrintf(db.ALWAYS, "MakeRealmResourceMgr")
	m := &RealmResourceMgr{}
	m.realmId = rid
	m.sigmaFsl = fslib.MakeFsLibAddr("realmmgr-sigmafsl", sigmaNamedAddrs)
	m.ConfigClnt = config.MakeConfigClnt(m.sigmaFsl)
	m.lock = electclnt.MakeElectClnt(m.sigmaFsl, path.Join(REALM_FENCES, rid), 0777)
	m.ec = electclnt.MakeElectClnt(m.sigmaFsl, path.Join(REALM_FENCES, rid+REALMMGR_ELECT), 0777)

	return m
}

func (m *RealmResourceMgr) makeCtlFiles() {
	// Set up control files
	ctl := makeCtlFile(m.receiveResourceGrant, m.handleResourceRequest, nil, m.Root())
	err := dir.MkNod(ctx.MkCtx("", 0, nil), m.Root(), realmctl, ctl)
	if err != nil {
		db.DFatalf("Error MkNod sigmactl: %v", err)
	}
}

func (m *RealmResourceMgr) receiveResourceGrant(msg *ResourceMsg) {
	switch msg.ResourceType {
	case Tnode:
		// Nothing much to do here, for now.
		db.DPrintf(db.ALWAYS, "Tnode granted")
	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

func (m *RealmResourceMgr) handleResourceRequest(msg *ResourceMsg) {
	switch msg.ResourceType {
	case Tnode:
		db.DPrintf(db.ALWAYS, "Tnode requested")
		lockRealm(m.lock, m.realmId)
		nodedId := m.getLeastUtilizedNoded()
		db.DPrintf(db.ALWAYS, "least utilized node: %v", nodedId)
		// Dealloc the Noded. The Noded will take care of registering itself as
		// free with the SigmaMgr.
		m.deallocNoded(nodedId)
		db.DPrintf(db.ALWAYS, "dealloced: %v", nodedId)
		unlockRealm(m.lock, m.realmId)
	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

// Deallocate a noded from a realm.
func (m *RealmResourceMgr) deallocNoded(nodedId string) {
	// Note noded de-registration
	rCfg := &RealmConfig{}
	m.ReadConfig(path.Join(REALM_CONFIG, m.realmId), rCfg)
	// Remove the noded from the list of assigned nodeds.
	for i := range rCfg.NodedsAssigned {
		if rCfg.NodedsAssigned[i] == nodedId {
			rCfg.NodedsAssigned = append(rCfg.NodedsAssigned[:i], rCfg.NodedsAssigned[i+1:]...)
		}
	}
	rCfg.LastResize = time.Now()
	m.WriteConfig(path.Join(REALM_CONFIG, m.realmId), rCfg)

	rdCfg := &NodedConfig{}
	rdCfg.Id = nodedId
	rdCfg.RealmId = kernel.NO_REALM

	// Update the noded config file.
	m.WriteConfig(path.Join(NODED_CONFIG, nodedId), rdCfg)
}

// XXX Do I really need this?
func (m *RealmResourceMgr) getRealmConfig() (*RealmConfig, error) {
	// If the realm is being shut down, the realm config file may not be there
	// anymore. In this case, another noded is not needed.
	if _, err := m.sigmaFsl.Stat(path.Join(REALM_CONFIG, m.realmId)); err != nil && strings.Contains(err.Error(), "file not found") {
		return nil, fmt.Errorf("Realm not found")
	}
	cfg := &RealmConfig{}
	m.ReadConfig(path.Join(REALM_CONFIG, m.realmId), cfg)
	return cfg, nil
}

// Get all a realm's procd's stats
func (m *RealmResourceMgr) getRealmProcdStats(nameds []string) map[string]*stats.StatInfo {
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

func (m *RealmResourceMgr) getRealmUtil(cfg *RealmConfig) (float64, map[string]float64) {
	// Get stats
	utilMap := make(map[string]float64)
	procdStats := m.getRealmProcdStats(cfg.NamedAddrs)
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

func (m *RealmResourceMgr) getLeastUtilizedNoded() string {
	// Get the realm's config
	realmCfg, err := m.getRealmConfig()
	if err != nil {
		db.DFatalf("Error getRealmConfig: %v", err)
	}

	_, procdUtils := m.getRealmUtil(realmCfg)
	db.DPrintf(db.ALWAYS, "searching for least utilized node, procd utils: %v", procdUtils)
	// Find least utilized procd
	min := 100.0
	minNodedId := ""
	for nodedId, util := range procdUtils {
		if min > util {
			min = util
			minNodedId = nodedId
		}
	}
	return minNodedId
}

func (m *RealmResourceMgr) adjustRealm() {
	// Get the realm's config
	realmCfg, err := m.getRealmConfig()
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

	avgUtil, _ := m.getRealmUtil(realmCfg)
	if avgUtil > np.REALM_GROW_CPU_UTIL_THRESHOLD {
		// TODO: request noded
	}
}

func (m *RealmResourceMgr) Work() {
	db.DPrintf(db.ALWAYS, "Realmmgr started")
	// Acquire primary role
	if err := m.ec.AcquireLeadership([]byte("realmmgr")); err != nil {
		db.DFatalf("Acquire leadership: %v", err)
	}

	// TODO: move to sigma named.
	// XXX Currently, because of this scheme in which each noded runs a realmmgr,
	// the sigmamgr and other requesting applications may have to retry on
	// TErrNotfound.
	var err error
	m.MemFs, m.FsLib, m.ProcClnt, err = fslibsrv.MakeMemFs(np.REALM_MGR, "realmmgr")
	if err != nil {
		db.DFatalf("Error MakeMemFs in MakeSigmaResourceMgr: %v", err)
	}

	m.makeCtlFiles()

	m.Started()

	for {
		//		lockRealm(m.lock, m.realmId)
		//		m.adjustRealm()
		//		unlockRealm(m.lock, m.realmId)

		// Sleep for a bit.
		time.Sleep(np.REALM_SCAN_INTERVAL_MS * time.Millisecond)
	}
}
