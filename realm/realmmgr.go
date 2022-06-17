package realm

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"ulambda/config"
	db "ulambda/debug"
	"ulambda/electclnt"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/kernel"
	"ulambda/machine"
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
	REALMMGR       = "realmmgr"
)

type RealmResourceMgr struct {
	realmId string
	// ===== Relative to the sigma named =====
	sigmaFsl *fslib.FsLib
	lock     *electclnt.ElectClnt
	*config.ConfigClnt
	memfs *fslibsrv.MemFs
	*procclnt.ProcClnt
	// ===== Relative to the realm named =====
	*fslib.FsLib
}

func MakeRealmResourceMgr(realmId string) *RealmResourceMgr {
	db.DPrintf("REALMMGR", "MakeRealmResourceMgr")
	m := &RealmResourceMgr{}
	m.realmId = realmId
	m.sigmaFsl = fslib.MakeFsLib(proc.GetPid().String() + "-sigmafsl")
	m.ProcClnt = procclnt.MakeProcClnt(m.sigmaFsl)
	m.ConfigClnt = config.MakeConfigClnt(m.sigmaFsl)
	m.lock = electclnt.MakeElectClnt(m.sigmaFsl, path.Join(REALM_FENCES, realmId), 0777)

	var err error
	m.memfs, err = fslibsrv.MakeMemFsFsl(path.Join(REALM_MGRS, m.realmId), m.sigmaFsl, m.ProcClnt)
	if err != nil {
		db.DFatalf("Error MakeMemFs in MakeSigmaResourceMgr: %v", err)
	}

	resource.MakeCtlFile(m.receiveResourceGrant, m.handleResourceRequest, m.memfs.Root(), np.RESOURCE_CTL)

	return m
}

func (m *RealmResourceMgr) receiveResourceGrant(msg *resource.ResourceMsg) {
	switch msg.ResourceType {
	case resource.Tcore:
		db.DPrintf("REALMMGR", "resource.Tcore granted")
		db.DPrintf(db.ALWAYS, "resource.Tcore granted")
		m.growRealm()
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

// This realm has been granted cores. Now grow it.
//
// TODO: First try to add cores to existing nodeds.
func (m *RealmResourceMgr) growRealm() {
	// Find a machine with free cores and claim them
	machineId, ok := m.getFreeCores(1)
	if !ok {
		db.DFatalf("Unable to get free cores to grow realm")
	}
	db.DPrintf(db.ALWAYS, "Start a new noded on %v", machineId)
	// Request the machine to start a noded.
	nodedId := m.requestNoded(machineId)
	db.DPrintf(db.ALWAYS, "Started noded %v on %v", nodedId, machineId)
	// Allocate the noded to this realm.
	allocNoded(m, m.lock, m.realmId, nodedId.String())
	db.DPrintf(db.ALWAYS, "Allocated %v to realm %v", nodedId, m.realmId)
}

func (m *RealmResourceMgr) getFreeCores(nRetries int) (string, bool) {
	lockRealm(m.lock, m.realmId)
	defer unlockRealm(m.lock, m.realmId)

	var machineId string
	for i := 0; i < nRetries; i++ {
		ok, err := m.sigmaFsl.ProcessDir(machine.MACHINES, func(st *np.Stat) (bool, error) {
			cdir := path.Join(machine.MACHINES, st.Name, machine.CORES)
			coreGroups, err := m.sigmaFsl.GetDir(cdir)
			if err != nil {
				db.DFatalf("Error GetDir: %v", err)
			}
			// Try to steal a core group
			for i := 0; i < len(coreGroups); i++ {
				err := m.sigmaFsl.Remove(path.Join(cdir, coreGroups[i].Name))
				if err == nil {
					machineId = st.Name
					return true, nil
				} else {
					if !np.IsErrNotfound(err) {
						return false, err
					}
				}
			}
			return false, nil
		})
		if err != nil {
			db.DFatalf("Error ProcessDir: %v", err)
		}
		// Successfully grew realm.
		if ok {
			return machineId, ok
		}
	}
	return "", false
}

// Request a machine to create a new Noded)
func (m *RealmResourceMgr) requestNoded(machineId string) proc.Tpid {
	pid := proc.Tpid("noded-" + proc.GenPid().String())
	msg := resource.MakeResourceMsg(resource.Trequest, resource.Tnode, pid.String(), 1)
	if _, err := m.sigmaFsl.SetFile(path.Join(machine.MACHINES, machineId, np.RESOURCE_CTL), msg.Marshal(), np.OWRITE, 0); err != nil {
		db.DFatalf("Error SetFile in requestNoded: %v", err)
	}
	return pid
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

	// If we don't have enough noded replicas to start the realm yet, we need to
	// grow the realm.
	if len(realmCfg.NodedsAssigned) < nReplicas() {
		return true
	}

	// If we haven't finished booting, we aren't ready to start scanning/growing
	// the realm.
	if len(realmCfg.NodedsActive) < nReplicas() {
		return false
	} else {
		// If the realm just finished booting, finish initialization.
		if m.FsLib == nil {
			m.FsLib = fslib.MakeFsLibAddr(proc.GetPid().String(), realmCfg.NamedAddrs)
		}
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

	for {
		if m.realmShouldGrow() {
			db.DPrintf("REALMMGR", "Try to grow realm %v", m.realmId)
			msg := resource.MakeResourceMsg(resource.Trequest, resource.Tcore, m.realmId, 1)
			if _, err := m.sigmaFsl.SetFile(path.Join(np.SIGMACTL), msg.Marshal(), np.OWRITE, 0); err != nil {
				db.DFatalf("Error SetFile: %v", err)
			}
		}
		// Sleep for a bit.
		time.Sleep(np.Conf.Realm.SCAN_INTERVAL)
	}
}
