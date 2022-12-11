package realm

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"sigmaos/config"
	db "sigmaos/debug"
	"sigmaos/electclnt"
	"sigmaos/fcall"
	"sigmaos/fslib"
	"sigmaos/machine"
	mproto "sigmaos/machine/proto"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
	"sigmaos/realm/proto"
	"sigmaos/resource"
	np "sigmaos/sigmap"
	"sigmaos/stats"
)

/* RealmMgr responsibilities:
 * - Respond to resource requests from SigmaMgr, and deallocate Nodeds.
 * - Ask for more Nodeds from SigmaMgr when load increases.
 */

type RealmResourceMgr struct {
	realmId string
	// ===== Relative to the sigma named =====
	sigmaFsl *fslib.FsLib
	lock     *electclnt.ElectClnt
	*config.ConfigClnt
	pds *protdevsrv.ProtDevSrv
	*procclnt.ProcClnt
	mclnts map[string]*protdevclnt.ProtDevClnt
	nclnts map[string]*protdevclnt.ProtDevClnt
	// ===== Relative to the realm named =====
	*fslib.FsLib
}

func MakeRealmResourceMgr(realmId string) *RealmResourceMgr {
	db.DPrintf("REALMMGR", "MakeRealmResourceMgr %v", realmId)
	m := &RealmResourceMgr{}
	m.realmId = realmId
	m.sigmaFsl = fslib.MakeFsLib(proc.GetPid().String() + "-sigmafsl")
	m.ProcClnt = procclnt.MakeProcClnt(m.sigmaFsl)
	m.ConfigClnt = config.MakeConfigClnt(m.sigmaFsl)
	m.lock = electclnt.MakeElectClnt(m.sigmaFsl, realmFencePath(realmId), 0777)
	m.mclnts = make(map[string]*protdevclnt.ProtDevClnt)
	m.nclnts = make(map[string]*protdevclnt.ProtDevClnt)

	mfs, err := memfssrv.MakeMemFsFsl(realmMgrPath(m.realmId), m.sigmaFsl, m.ProcClnt)
	if err != nil {
		db.DFatalf("Error MakeMemFs in MakeSigmaResourceMgr: %v", err)
	}

	m.pds, err = protdevsrv.MakeProtDevSrvMemFs(mfs, m)
	if err != nil {
		db.DFatalf("Error PDS: %v", err)
	}

	m.initFS()

	return m
}

func (m *RealmResourceMgr) initFS() {
	dirs := []string{NODEDS}
	for _, d := range dirs {
		if err := m.sigmaFsl.MkDir(path.Join(realmMgrPath(m.realmId), d), 0777); err != nil {
			db.DFatalf("Error Mkdir: %v", err)
		}
	}
}

func (m *RealmResourceMgr) GrantCores(req proto.RealmMgrRequest, res *proto.RealmMgrResponse) error {
	db.DPrintf("REALMMGR", "[%v] resource.Tcore granted %v", m.realmId, req.Ncores)
	m.growRealm(int(req.Ncores))
	res.OK = true
	return nil
}

// XXX Should we prioritize defragmentation, or try to avoid evictions?
func (m *RealmResourceMgr) RevokeCores(req proto.RealmMgrRequest, res *proto.RealmMgrResponse) error {
	lockRealm(m.lock, m.realmId)
	defer unlockRealm(m.lock, m.realmId)

	db.DPrintf("REALMMGR", "[%v] resource.Tcore requested", m.realmId)

	nodedId, ok := m.getLeastUtilizedNoded()

	// If no Nodeds are underutilized...
	if !ok {
		return nil
	}

	db.DPrintf("REALMMGR", "[%v] least utilized node: %v", m.realmId, nodedId)

	// Read this noded's config.
	ndCfg := &NodedConfig{}
	m.ReadConfig(NodedConfPath(nodedId), ndCfg)

	cores := ndCfg.Cores[len(ndCfg.Cores)-1]
	db.DPrintf("REALMMGR", "[%v] Revoking cores %v from noded %v", m.realmId, cores, nodedId)
	// Otherwise, take some cores away.
	nres := &proto.NodedResponse{}
	nreq := &proto.NodedRequest{
		Cores: cores,
	}
	err := m.nclnts[nodedId].RPC("Noded.RevokeCores", nreq, nres)
	if err != nil || !nres.OK {
		db.DFatalf("Error RPC: %v %v", err, nres.OK)
	}
	db.DPrintf("REALMMGR", "[%v] Revoked cores %v from noded %v", m.realmId, cores, nodedId)
	m.updateResizeTimeL(m.realmId)
	res.OK = true
	return nil
}

func (m *RealmResourceMgr) ShutdownRealm(req proto.RealmMgrRequest, res *proto.RealmMgrResponse) error {
	lockRealm(m.lock, m.realmId)
	defer unlockRealm(m.lock, m.realmId)

	// Sanity check
	if !req.AllCores {
		db.DFatalf("Shutdown realm without asking for all cores")
	}

	// On realm request, shut down & kill all nodeds.
	db.DPrintf("REALMMGR", "[%v] Realm shutdown requested", m.realmId)
	realmCfg, err := m.getRealmConfig()
	if err != nil {
		db.DFatalf("Error get realm config.")
	}
	for _, nodedId := range realmCfg.NodedsAssigned {
		nres := &proto.NodedResponse{}
		nreq := &proto.NodedRequest{
			AllCores: true,
		}
		err := m.nclnts[nodedId].RPC("Noded.RevokeCores", nreq, nres)
		if err != nil || !nres.OK {
			db.DFatalf("Error RPC: %v %v", err, nres.OK)
		}
		db.DPrintf("REALMMGR", "[%v] Deallocating noded %v", m.realmId, nodedId)
	}
	res.OK = true
	return nil
}

// This realm has been granted cores. Now grow it. Sigmamgr must hold lock.
func (m *RealmResourceMgr) growRealm(amt int) {
	// Find a machine with free cores and claim them
	machineIds, nodedIds, cores, ok := m.getFreeCores(amt)
	if !ok {
		db.DFatalf("Unable to get free cores to grow realm %v", amt)
	}
	// For each machine, allocate some cores.
	for i := range machineIds {
		// Allocate each core claimed on this machine, one at a time.
		for _, c := range cores[i] {
			amt--
			// If there was no noded from this realm already running on this
			// machine...
			if nodedIds[i] == "" {
				db.DPrintf("REALMMGR", "[%v] Start a new noded on %v with cores %v", m.realmId, machineIds[i], cores)
				nodedIds[i] = proc.Tpid("noded-" + proc.GenPid().String()).String()
				// Allocate the noded to this realm by creating its config file. This
				// must happen before actually requesting the noded, so we don't
				// deadlock.
				db.DPrintf("REALMMGR", "[%v] Allocating %v to realm", m.realmId, nodedIds[i])
				m.allocNoded(m.realmId, machineIds[i], nodedIds[i], c)
				db.DPrintf("REALMMGR", "[%v] Requesting noded %v", m.realmId, nodedIds[i])
				// Request the machine to start a noded.
				m.requestNoded(nodedIds[i], machineIds[i])
				db.DPrintf("REALMMGR", "[%v] Started noded %v on %v", m.realmId, nodedIds[i], machineIds[i])
				db.DPrintf("REALMMGR", "[%v] Allocated %v", m.realmId, nodedIds[i])
				var err error
				m.nclnts[nodedIds[i]], err = protdevclnt.MkProtDevClnt(m.sigmaFsl, nodedPath(m.realmId, nodedIds[i]))
				if err != nil {
					db.DFatalf("Error MkProtDevClnt: %v", err)
				}
			} else {
				db.DPrintf("REALMMGR", "[%v] Growing noded %v core allocation on machine %v by %v", m.realmId, nodedIds[i], machineIds[i], cores)
				res := &proto.NodedResponse{}
				req := &proto.NodedRequest{
					Cores: c,
				}
				err := m.nclnts[nodedIds[i]].RPC("Noded.GrantCores", req, res)
				if err != nil || !res.OK {
					db.DFatalf("Error RPC: %v %v", err, res.OK)
				}
			}
			m.updateResizeTime(m.realmId)
		}
	}
	if amt > 0 {
		db.DFatalf("Grew realm, but not by enough %v", amt)
	}
}

func (m *RealmResourceMgr) tryClaimCores(machineId string, amt int) ([]*np.Tinterval, bool) {
	cdir := path.Join(machine.MACHINES, machineId, machine.CORES)
	coreGroups, err := m.sigmaFsl.GetDir(cdir)
	if err != nil {
		db.DFatalf("Error GetDir: %v", err)
	}
	cores := make([]*np.Tinterval, 0)
	// Try to steal a core group
	for i := 0; i < len(coreGroups) && amt > 0; i++ {
		// Read the core file.
		coreFile := path.Join(cdir, coreGroups[i].Name)
		// Claim the cores
		err = m.sigmaFsl.Remove(coreFile)
		if err == nil {
			// Cores successfully claimed.
			c := np.MkInterval(0, 0)
			c.Unmarshal(string(coreGroups[i].Name))
			cores = append(cores, c)
			amt--
		} else {
			// Unexpected error
			if !fcall.IsErrNotfound(err) {
				db.DFatalf("Error Remove %v", err)
			}
		}
	}
	return cores, len(cores) > 0
}

func (m *RealmResourceMgr) getFreeCores(amt int) ([]string, []string, [][]*np.Tinterval, bool) {
	lockRealm(m.lock, m.realmId)
	defer unlockRealm(m.lock, m.realmId)

	machineIds := make([]string, 0)
	nodedIds := make([]string, 0)
	cores := make([][]*np.Tinterval, 0)
	var err error

	// First, try to get cores on a machine already running a noded from this
	// realm.
	_, err = m.sigmaFsl.ProcessDir(path.Join(realmMgrPath(m.realmId), NODEDS), func(nd *np.Stat) (bool, error) {
		ndCfg := MakeNodedConfig()
		m.ReadConfig(NodedConfPath(nd.Name), ndCfg)
		// Try to claim additional cores on the machine this noded lives on.
		if c, ok := m.tryClaimCores(ndCfg.MachineId, amt); ok {
			cores = append(cores, c)
			machineIds = append(machineIds, ndCfg.MachineId)
			nodedIds = append(nodedIds, nd.Name)
			amt -= len(c)
		}
		if amt == 0 {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		db.DFatalf("Error ProcessDir: %v", err)
	}
	// If successfully claimed enough cores, return
	if amt == 0 {
		return machineIds, nodedIds, cores, true
	}
	// Otherwise, Try to get cores on any machine.
	_, err = m.sigmaFsl.ProcessDir(machine.MACHINES, func(st *np.Stat) (bool, error) {
		if c, ok := m.tryClaimCores(st.Name, amt); ok {
			cores = append(cores, c)
			machineIds = append(machineIds, st.Name)
			nodedIds = append(nodedIds, "")
			amt -= len(c)
		}
		if amt == 0 {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		db.DFatalf("Error ProcessDir: %v", err)
	}
	return machineIds, nodedIds, cores, amt == 0
}

// Request a machine to create a new Noded)
func (m *RealmResourceMgr) requestNoded(nodedId string, machineId string) {
	var clnt *protdevclnt.ProtDevClnt
	var ok bool
	if clnt, ok = m.mclnts[machineId]; !ok {
		var err error
		clnt, err = protdevclnt.MkProtDevClnt(m.sigmaFsl, path.Join(machine.MACHINES, machineId))
		if err != nil {
			db.DFatalf("Error MkProtDevClnt: %v", err)
		}
		m.mclnts[machineId] = clnt
	}

	res := &mproto.MachineResponse{}
	req := &mproto.MachineRequest{
		NodedId: nodedId,
	}
	err := clnt.RPC("Machined.BootNoded", req, res)
	if err != nil || !res.OK {
		db.DFatalf("Error RPC: %v %v", err, res.OK)
	}
}

// Alloc a Noded to this realm.
func (m *RealmResourceMgr) allocNoded(realmId, machineId, nodedId string, cores *np.Tinterval) {
	// Update the noded's config
	ndCfg := MakeNodedConfig()
	ndCfg.Id = nodedId
	ndCfg.RealmId = realmId
	ndCfg.MachineId = machineId
	ndCfg.Cores = append(ndCfg.Cores, cores)
	m.WriteConfig(NodedConfPath(nodedId), ndCfg)

	lockRealm(m.lock, realmId)
	defer unlockRealm(m.lock, realmId)

	// Update the realm's config
	rCfg := &RealmConfig{}
	m.ReadConfig(RealmConfPath(realmId), rCfg)
	rCfg.NodedsAssigned = append(rCfg.NodedsAssigned, nodedId)
	m.WriteConfig(RealmConfPath(realmId), rCfg)
}

func (m *RealmResourceMgr) updateResizeTime(realmId string) {
	lockRealm(m.lock, realmId)
	defer unlockRealm(m.lock, realmId)

	m.updateResizeTimeL(realmId)
}

func (m *RealmResourceMgr) updateResizeTimeL(realmId string) {
	// Update the realm's config
	rCfg := &RealmConfig{}
	m.ReadConfig(RealmConfPath(realmId), rCfg)
	rCfg.LastResize = time.Now()
	m.WriteConfig(RealmConfPath(realmId), rCfg)
}

// XXX Do I really need this?
func (m *RealmResourceMgr) getRealmConfig() (*RealmConfig, error) {
	// If the realm is being shut down, the realm config file may not be there
	// anymore. In this case, another noded is not needed.
	if _, err := m.sigmaFsl.Stat(RealmConfPath(m.realmId)); err != nil && strings.Contains(err.Error(), "file not found") {
		return nil, fmt.Errorf("Realm not found")
	}
	cfg := &RealmConfig{}
	m.ReadConfig(RealmConfPath(m.realmId), cfg)
	return cfg, nil
}

// Get all a realm's procd's stats
func (m *RealmResourceMgr) getRealmProcdStats(nodeds []string) map[string]*stats.StatInfo {
	stat := make(map[string]*stats.StatInfo)
	for _, nodedId := range nodeds {
		ndCfg := MakeNodedConfig()
		m.ReadConfig(NodedConfPath(nodedId), ndCfg)
		s := &stats.StatInfo{}
		err := m.GetFileJson(path.Join(np.PROCD, ndCfg.ProcdIp, np.STATSD), s)
		if err != nil {
			db.DPrintf("REALMMGR_ERR", "[%v] Error ReadFileJson in SigmaResourceMgr.getRealmProcdStats: %v", m.realmId, err)
			continue
		}
		stat[nodedId] = s
	}
	return stat
}

func (m *RealmResourceMgr) getRealmUtil(cfg *RealmConfig) (float64, map[string]float64) {
	// Get stats
	utilMap := make(map[string]float64)
	procdStats := m.getRealmProcdStats(cfg.NodedsActive)
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

func (m *RealmResourceMgr) getRealmQueueLen() int {
	sts1, _ := m.GetDir(path.Join(np.PROCD_WS, np.PROCD_RUNQ_LC))
	sts2, _ := m.GetDir(path.Join(np.PROCD_WS, np.PROCD_RUNQ_BE))
	return len(sts1) + len(sts2)
}

func (m *RealmResourceMgr) getLeastUtilizedNoded() (string, bool) {
	// Get the realm's config
	realmCfg, err := m.getRealmConfig()
	if err != nil {
		db.DFatalf("Error getRealmConfig: %v", err)
	}

	if len(realmCfg.NamedAddrs) == 0 {
		return "", false
	}

	_, procdUtils := m.getRealmUtil(realmCfg)
	db.DPrintf("REALMMGR", "[%v] searching for least utilized node, procd utils: %v", m.realmId, procdUtils)

	nodeds := make([]string, 0, len(procdUtils))
	for nodedId, _ := range procdUtils {
		nodeds = append(nodeds, nodedId)
	}
	// Sort nodeds in order of ascending utilization.
	sort.Slice(nodeds, func(i, j int) bool {
		return procdUtils[nodeds[i]] < procdUtils[nodeds[j]]
	})
	// Find a noded which can be shrunk.
	for _, nodedId := range nodeds {
		if nodedOverprovisioned(m.sigmaFsl, m.ConfigClnt, m.realmId, nodedId, "REALMMGR") {
			return nodedId, true
		}
	}
	return "", false
}

// Returns true if the realm should grow, and returns the queue length.
func (m *RealmResourceMgr) realmShouldGrow() (int, bool) {
	lockRealm(m.lock, m.realmId)
	defer unlockRealm(m.lock, m.realmId)

	// Get the realm's config
	realmCfg, err := m.getRealmConfig()
	if err != nil {
		db.DPrintf("REALMMGR", "Error getRealmConfig: %v", err)
		return 0, false
	}

	// If the realm is shutting down, return
	if realmCfg.Shutdown {
		return 0, false
	}

	// If we don't have enough noded replicas to start the realm yet, we need to
	// grow the realm.
	if len(realmCfg.NodedsAssigned) < nReplicas() {
		return 1, true
	}

	// If we haven't finished booting, we aren't ready to start scanning/growing
	// the realm.
	if len(realmCfg.NodedsActive) < nReplicas() {
		return 0, false
	} else {
		// If the realm just finished booting, finish initialization.
		if m.FsLib == nil {
			m.FsLib = fslib.MakeFsLibAddr(proc.GetPid().String(), realmCfg.NamedAddrs)
		}
	}

	// If we have resized too recently, return
	if time.Now().Sub(realmCfg.LastResize) < np.Conf.Realm.RESIZE_INTERVAL {
		return 0, false
	}

	// If there are a lot of procs waiting to be run/stolen...
	qlen := m.getRealmQueueLen()
	if qlen >= int(machine.NodedNCores()) {
		return qlen, true
	}

	avgUtil, _ := m.getRealmUtil(realmCfg)

	if avgUtil > np.Conf.Realm.GROW_CPU_UTIL_THRESHOLD && qlen >= 0 {
		return qlen, true
	}
	return 0, false
}

func (m *RealmResourceMgr) Work() {
	db.DPrintf("REALMMGR", "Realmmgr started in realm %v", m.realmId)

	m.Started()
	go func() {
		m.WaitEvict(proc.GetPid())
		db.DPrintf("REALMMGR", "[%v] Evicted!", m.realmId)
		m.Exited(proc.MakeStatus(proc.StatusEvicted))
		db.DPrintf("REALMMGR", "[%v] Exited", m.realmId)
		os.Exit(0)
	}()

	for {
		if qlen, ok := m.realmShouldGrow(); ok {
			db.DPrintf("REALMMGR", "[%v] Try to grow realm qlen %v", m.realmId, qlen)
			msg := resource.MakeResourceMsg(resource.Trequest, resource.Tcore, m.realmId, qlen)
			resource.SendMsg(m.sigmaFsl, np.SIGMACTL, msg)
		}
		// Sleep for a bit.
		time.Sleep(np.Conf.Realm.SCAN_INTERVAL)
	}
}
