package realm

import (
	"fmt"
	"os"
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
	"ulambda/resource"
	"ulambda/stats"
)

/* RealmMgr responsibilities:
 * - Respond to resource requests from SigmaMgr, and deallocate Nodeds.
 * - Ask for more Nodeds from SigmaMgr when load increases.
 */

const (
	REALMMGR_ELECT = "-realmmgr-elect"
	realmctl       = "realmctl"
	REALMMGR       = "realmmgr"
)

type RealmResourceMgr struct {
	realmId string
	// ===== Relative to the sigma named =====
	sigmaFsl *fslib.FsLib
	ec       *electclnt.ElectClnt
	lock     *electclnt.ElectClnt
	*config.ConfigClnt
	memfs *fslibsrv.MemFs
	*procclnt.ProcClnt
	// ===== Relative to the realm named =====
	*fslib.FsLib
}

// TODO: Make this proc un-stealable
func MakeRealmResourceMgr(rid string, realmNamedAddrs []string) *RealmResourceMgr {
	db.DPrintf("REALMMGR", "MakeRealmResourceMgr")
	m := &RealmResourceMgr{}
	m.realmId = rid
	m.sigmaFsl = fslib.MakeFsLib(proc.GetPid().String() + "-sigmafsl")
	m.FsLib = fslib.MakeFsLibAddr(proc.GetPid().String(), realmNamedAddrs)
	m.ProcClnt = procclnt.MakeProcClnt(m.sigmaFsl)
	m.ConfigClnt = config.MakeConfigClnt(m.sigmaFsl)
	m.lock = electclnt.MakeElectClnt(m.sigmaFsl, path.Join(REALM_FENCES, rid), 0777)
	m.ec = electclnt.MakeElectClnt(m.sigmaFsl, path.Join(REALM_FENCES, rid+REALMMGR_ELECT), 0777)

	return m
}

func (m *RealmResourceMgr) makeCtlFiles() {
	// Set up control files
	ctl := makeCtlFile(m.receiveResourceGrant, m.handleResourceRequest, nil, m.memfs.Root())
	err := dir.MkNod(ctx.MkCtx("", 0, nil), m.memfs.Root(), realmctl, ctl)
	if err != nil {
		db.DFatalf("Error MkNod sigmactl: %v", err)
	}
}

func (m *RealmResourceMgr) receiveResourceGrant(msg *resource.ResourceMsg) {
	switch msg.ResourceType {
	case resource.Tnode:
		// Nothing much to do here, for now.
		db.DPrintf("REALMMGR", "resource.Tnode granted")
	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

func (m *RealmResourceMgr) handleResourceRequest(msg *resource.ResourceMsg) {
	switch msg.ResourceType {
	case resource.Tnode:
		db.DPrintf("REALMMGR", "resource.Tnode requested")
		lockRealm(m.lock, m.realmId)
		nodedId := m.getLeastUtilizedNoded()
		db.DPrintf("REALMMGR", "least utilized node: %v", nodedId)
		// If no Nodeds remain...
		if nodedId == "" {
			return
		}
		// Dealloc the Noded. The Noded will take care of registering itself as
		// free with the SigmaMgr.
		m.deallocNoded(nodedId)
		db.DPrintf("REALMMGR", "dealloced: %v", nodedId)
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
	db.DPrintf("REALMMGR", "Dealloc noded, choosing from: %v", rCfg.NodedsAssigned)
	// Remove the noded from the list of assigned nodeds.
	for i := range rCfg.NodedsAssigned {
		if rCfg.NodedsAssigned[i] == nodedId {
			rCfg.NodedsAssigned = append(rCfg.NodedsAssigned[:i], rCfg.NodedsAssigned[i+1:]...)
			break
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
				db.DPrintf("REALMMGR", "Error ReadFileJson in SigmaResourceMgr.getRealmProcdStats: %v", err)
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
	db.DPrintf("REALMMGR", "searching for least utilized node, procd utils: %v", procdUtils)
	// Find least utilized procd
	min := 100.0
	minNodedId := ""
	for nodedId, util := range procdUtils {
		if min >= util {
			min = util
			minNodedId = nodedId
		}
	}
	return minNodedId
}

func (m *RealmResourceMgr) realmShouldGrow() bool {
	lockRealm(m.lock, m.realmId)
	defer unlockRealm(m.lock, m.realmId)

	// Get the realm's config
	realmCfg, err := m.getRealmConfig()
	if err != nil {
		db.DPrintf("REALMMGR", "Error getRealmConfig: %v", err)
		return false
	}

	// If the realm is shutting down, return
	if realmCfg.Shutdown {
		return false
	}

	// If we have resized too recently, return
	if time.Now().Sub(realmCfg.LastResize) < np.Conf.Realm.RESIZE_INTERVAL {
		return false
	}

	avgUtil, _ := m.getRealmUtil(realmCfg)

	if avgUtil > np.Conf.Realm.GROW_CPU_UTIL_THRESHOLD {
		return true
	}
	return false
}

func (m *RealmResourceMgr) Work() {
	db.DPrintf("REALMMGR", "Realmmgr started")

	m.Started()
	go func() {
		m.WaitEvict(proc.GetPid())
		db.DPrintf("REALMMGR", "Evicted!")
		m.Exited(proc.MakeStatus(proc.StatusEvicted))
		db.DPrintf("REALMMGR", "Exited")
		os.Exit(0)
	}()

	// Acquire primary role
	if err := m.ec.AcquireLeadership([]byte(REALMMGR)); err != nil {
		db.DFatalf("Acquire leadership: %v", err)
	}

	// TODO: move realm fs files to sigma named.
	// XXX Currently, because of this scheme in which each noded runs a realmmgr,
	// the sigmamgr and other requesting applications may have to retry on
	// TErrNotfound.
	var err error
	m.memfs, err = fslibsrv.MakeMemFsFsl(path.Join(REALM_MGRS, m.realmId), m.sigmaFsl, m.ProcClnt)
	if err != nil {
		db.DFatalf("Error MakeMemFs in MakeSigmaResourceMgr: %v", err)
	}

	m.makeCtlFiles()

	for {
		if m.realmShouldGrow() {
			db.DPrintf("REALMMGR", "Try to grow realm %v", m.realmId)
			msg := resource.MakeResourceMsg(resource.Trequest, resource.Tnode, m.realmId, 1)
			if _, err := m.sigmaFsl.SetFile(path.Join(SIGMACTL), msg.Marshal(), np.OWRITE, 0); err != nil {
				db.DFatalf("Error SetFile: %v", err)
			}
		}

		// Sleep for a bit.
		time.Sleep(np.Conf.Realm.SCAN_INTERVAL)
	}
}
