package realm

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
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
	sp "sigmaos/sigmap"
	"sigmaos/stats"
)

/* RealmMgr responsibilities:
 * - Respond to resource requests from SigmaMgr, and deallocate Nodeds.
 * - Ask for more Nodeds from SigmaMgr when load increases.
 */

type RealmResourceMgr struct {
	sync.Mutex
	realmId string
	// ===== Relative to the sigma named =====
	sigmaFsl *fslib.FsLib
	lock     *electclnt.ElectClnt
	*config.ConfigClnt
	pds *protdevsrv.ProtDevSrv
	*procclnt.ProcClnt
	nodedToMachined map[string]string
	mclnts          map[string]*protdevclnt.ProtDevClnt
	nclnts          map[string]*protdevclnt.ProtDevClnt
	sclnt           *protdevclnt.ProtDevClnt
	lastGrow        time.Time
	// ===== Relative to the realm named =====
	*fslib.FsLib
}

func MakeRealmResourceMgr(realmId string) *RealmResourceMgr {
	db.DPrintf(db.REALMMGR, "MakeRealmResourceMgr %v", realmId)
	m := &RealmResourceMgr{}
	m.realmId = realmId
	m.sigmaFsl = fslib.MakeFsLib(proc.GetPid().String() + "-sigmafsl")
	m.ProcClnt = procclnt.MakeProcClnt(m.sigmaFsl)
	m.ConfigClnt = config.MakeConfigClnt(m.sigmaFsl)
	m.lock = electclnt.MakeElectClnt(m.sigmaFsl, realmFencePath(realmId), 0777)
	m.mclnts = make(map[string]*protdevclnt.ProtDevClnt)
	m.nclnts = make(map[string]*protdevclnt.ProtDevClnt)
	m.nodedToMachined = make(map[string]string, 0)

	mfs, err := memfssrv.MakeMemFsFsl(realmMgrPath(m.realmId), m.sigmaFsl, m.ProcClnt)
	if err != nil {
		db.DFatalf("Error MakeMemFs in MakeSigmaResourceMgr: %v", err)
	}

	m.pds, err = protdevsrv.MakeProtDevSrvMemFs(mfs, m)
	if err != nil {
		db.DFatalf("Error PDS: %v", err)
	}

	m.sclnt, err = protdevclnt.MkProtDevClnt(m.sigmaFsl, sp.SIGMAMGR)
	if err != nil {
		db.DFatalf("Error MkProtDevClnt: %v", err)
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
	db.DPrintf(db.REALMMGR, "[%v] resource.Tcore granted %v", m.realmId, req.Ncores)
	m.growRealm(int(req.Ncores))
	res.OK = true
	return nil
}

// XXX Should we prioritize defragmentation, or try to avoid evictions?
func (m *RealmResourceMgr) RevokeCores(req proto.RealmMgrRequest, res *proto.RealmMgrResponse) error {
	lockRealm(m.lock, m.realmId)
	defer unlockRealm(m.lock, m.realmId)

	res.OK = true

	// Get the realm's config
	realmCfg, err := m.getRealmConfig()
	if err != nil {
		db.DFatalf("Error getRealmConfig: %v", err)
	}

	// Don't revoke cores too quickly unless this is a hard request.
	if time.Now().Sub(realmCfg.LastResize) < sp.Conf.Realm.RESIZE_INTERVAL*3 && !req.HardReq {
		db.DPrintf(db.REALMMGR, "[%v] Soft core revocation request failed, resize too soon", m.realmId)
		res.OK = false
		return nil
	}

	nodedId := req.NodedId
	db.DPrintf(db.REALMMGR, "[%v] Core revoke request: %v hardReq %v", m.realmId, req.NodedId, req.HardReq)
	var ok bool
	if nodedId == "" {
		// If no requester preference, find the least utilized noded.
		nodedId, ok = m.getLeastUtilizedNoded()
	} else {
		// If requester has a preference, check if this noded is overprovisioned.
		_, _, ok = nodedOverprovisioned(m.sigmaFsl, m.ConfigClnt, m.realmId, nodedId, db.REALMMGR)
		db.DPrintf(db.REALMMGR, "[%v] Tried to satisfy (hard:%v) req for %v, result: %v ", m.realmId, req.HardReq, nodedId, ok)
	}

	// If no Nodeds are underutilized...
	if !ok {
		res.OK = false
		return nil
	}
	db.DPrintf(db.REALMMGR, "[%v] core revoked from least utilized node %v", m.realmId, nodedId)

	// Read this noded's config.
	ndCfg := &NodedConfig{}
	m.ReadConfig(NodedConfPath(nodedId), ndCfg)

	cores := ndCfg.Cores[len(ndCfg.Cores)-1]
	db.DPrintf(db.REALMMGR, "[%v] Revoking cores %v from noded %v", m.realmId, cores, nodedId)
	// Otherwise, take some cores away.
	nres := &proto.NodedResponse{}
	nreq := &proto.NodedRequest{
		Cores: cores,
	}
	m.Lock()
	clnt := m.nclnts[nodedId]
	m.Unlock()
	err = clnt.RPC("Noded.RevokeCores", nreq, nres)
	if err != nil || !nres.OK {
		db.DFatalf("Error RPC: %v %v", err, nres.OK)
	}
	db.DPrintf(db.REALMMGR, "[%v] Revoked cores %v from noded %v", m.realmId, cores, nodedId)
	m.updateResizeTimeL(m.realmId)
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
	db.DPrintf(db.REALMMGR, "[%v] Realm shutdown requested", m.realmId)
	realmCfg, err := m.getRealmConfig()
	if err != nil {
		db.DFatalf("Error get realm config.")
	}
	for _, nodedId := range realmCfg.NodedsAssigned {
		nres := &proto.NodedResponse{}
		nreq := &proto.NodedRequest{
			AllCores: true,
		}
		m.Lock()
		clnt := m.nclnts[nodedId]
		m.Unlock()
		err := clnt.RPC("Noded.RevokeCores", nreq, nres)
		if err != nil || !nres.OK {
			db.DFatalf("Error RPC: %v %v", err, nres.OK)
		}
		db.DPrintf(db.REALMMGR, "[%v] Deallocating noded %v", m.realmId, nodedId)
	}
	res.OK = true
	return nil
}

// This realm has been granted cores. Now grow it. Sigmamgr must hold lock.
func (m *RealmResourceMgr) growRealm(amt int) {
	defer func() {
		m.Lock()
		m.lastGrow = time.Now()
		m.Unlock()
		m.updateResizeTime(m.realmId)
	}()
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
				db.DPrintf(db.REALMMGR, "[%v] Start a new noded on %v with cores %v", m.realmId, machineIds[i], cores)
				nodedIds[i] = proc.Tpid("noded-" + proc.GenPid().String()).String()
				// Allocate the noded to this realm by creating its config file. This
				// must happen before actually requesting the noded, so we don't
				// deadlock.
				db.DPrintf(db.REALMMGR, "[%v] Allocating %v to realm", m.realmId, nodedIds[i])
				m.allocNoded(m.realmId, machineIds[i], nodedIds[i], c)
				db.DPrintf(db.REALMMGR, "[%v] Requesting noded %v", m.realmId, nodedIds[i])
				// Request the machine to start a noded.
				m.requestNoded(nodedIds[i], machineIds[i])
				db.DPrintf(db.REALMMGR, "[%v] Started noded %v on %v", m.realmId, nodedIds[i], machineIds[i])
				db.DPrintf(db.REALMMGR, "[%v] Allocated %v", m.realmId, nodedIds[i])
				clnt, err := protdevclnt.MkProtDevClnt(m.sigmaFsl, nodedPath(m.realmId, nodedIds[i]))
				m.Lock()
				m.nclnts[nodedIds[i]] = clnt
				m.Unlock()
				if err != nil {
					db.DFatalf("Error MkProtDevClnt: %v", err)
				}
			} else {
				db.DPrintf(db.REALMMGR, "[%v] Growing noded %v core allocation on machine %v by %v", m.realmId, nodedIds[i], machineIds[i], cores)
				res := &proto.NodedResponse{}
				req := &proto.NodedRequest{
					Cores: c,
				}
				m.Lock()
				clnt := m.nclnts[nodedIds[i]]
				m.Unlock()
				err := clnt.RPC("Noded.GrantCores", req, res)
				if err != nil || !res.OK {
					db.DFatalf("Error RPC: %v %v", err, res.OK)
				}
			}
		}
	}
	if amt > 0 {
		db.DFatalf("Grew realm, but not by enough %v", amt)
	}
}

func (m *RealmResourceMgr) tryClaimCores(machineId string, amt int) ([]*fcall.Tinterval, bool) {
	cdir := path.Join(machine.MACHINES, machineId, machine.CORES)
	coreGroups, err := m.sigmaFsl.GetDir(cdir)
	if err != nil {
		db.DFatalf("Error GetDir: %v", err)
	}
	cores := make([]*fcall.Tinterval, 0)
	// Try to steal a core group
	for i := 0; i < len(coreGroups) && amt > 0; i++ {
		// Read the core file.
		coreFile := path.Join(cdir, coreGroups[i].Name)
		// Claim the cores
		err = m.sigmaFsl.Remove(coreFile)
		if err == nil {
			// Cores successfully claimed.
			c := fcall.MkInterval(0, 0)
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

func (m *RealmResourceMgr) getFreeCores(amt int) ([]string, []string, [][]*fcall.Tinterval, bool) {
	lockRealm(m.lock, m.realmId)
	defer unlockRealm(m.lock, m.realmId)

	machineIds := make([]string, 0)
	nodedIds := make([]string, 0)
	cores := make([][]*fcall.Tinterval, 0)
	var err error

	// First, try to get cores on a machine already running a noded from this
	// realm.
	_, err = m.sigmaFsl.ProcessDir(path.Join(realmMgrPath(m.realmId), NODEDS), func(nd *sp.Stat) (bool, error) {
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
	_, err = m.sigmaFsl.ProcessDir(machine.MACHINES, func(st *sp.Stat) (bool, error) {
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
	m.Lock()
	if clnt, ok = m.mclnts[machineId]; !ok {
		var err error
		clnt, err = protdevclnt.MkProtDevClnt(m.sigmaFsl, path.Join(machine.MACHINES, machineId))
		if err != nil {
			db.DFatalf("Error MkProtDevClnt: %v", err)
		}
		m.mclnts[machineId] = clnt
	}
	m.Unlock()

	m.Lock()
	m.nodedToMachined[nodedId] = machineId
	m.Unlock()

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
func (m *RealmResourceMgr) allocNoded(realmId, machineId, nodedId string, cores *fcall.Tinterval) {
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
func (m *RealmResourceMgr) getRealmProcdStats(nodeds []string) (stat map[string]*stats.StatInfo) {
	stat = make(map[string]*stats.StatInfo)
	for _, nodedId := range nodeds {
		ndCfg := MakeNodedConfig()
		m.ReadConfig(NodedConfPath(nodedId), ndCfg)
		s := &stats.StatInfo{}
		err := m.GetFileJson(path.Join(sp.PROCD, ndCfg.ProcdIp, sp.STATSD), s)
		if err != nil {
			db.DPrintf(db.REALMMGR_ERR, "[%v] Error ReadFileJson in SigmaResourceMgr.getRealmProcdStats: %v", m.realmId, err)
			continue
		}
		stat[nodedId] = s
		// Map fron nodedId to machineId
	}
	return stat
}

func (m *RealmResourceMgr) getRealmUtil(cfg *RealmConfig) (avgUtil float64, utilMap map[string]float64, anyLC bool) {
	// Get stats
	utilMap = make(map[string]float64)
	var procdStats map[string]*stats.StatInfo
	procdStats = m.getRealmProcdStats(cfg.NodedsActive)
	avgUtil = 0.0
	anyLC = false
	for nodedId, stat := range procdStats {
		avgUtil += stat.Util
		utilMap[nodedId] = stat.Util
		// Procd stores LC proc utilization in the CustomUtil field of Stats.
		anyLC = anyLC || stat.CustomUtil > 0.0
	}
	if len(procdStats) > 0 {
		avgUtil /= float64(len(procdStats))
	}
	return avgUtil, utilMap, anyLC
}

func (m *RealmResourceMgr) getRealmQueueLen() (lcqlen int, beqlen int) {
	stslc, _ := m.GetDir(path.Join(sp.PROCD_WS, sp.PROCD_RUNQ_LC))
	stsbe, _ := m.GetDir(path.Join(sp.PROCD_WS, sp.PROCD_RUNQ_BE))
	return len(stslc), len(stsbe)
}

func sortNodedsByAscendingProcdUtil(procdUtils map[string]float64) []string {
	nodeds := make([]string, 0, len(procdUtils))
	for nodedId, _ := range procdUtils {
		nodeds = append(nodeds, nodedId)
	}
	// Sort nodeds in order of ascending utilization.
	sort.Slice(nodeds, func(i, j int) bool {
		return procdUtils[nodeds[i]] < procdUtils[nodeds[j]]
	})
	return nodeds
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

	_, procdUtils, _ := m.getRealmUtil(realmCfg)
	db.DPrintf(db.REALMMGR, "[%v] searching for least utilized node, procd utils: %v", m.realmId, procdUtils)

	nodeds := sortNodedsByAscendingProcdUtil(procdUtils)
	// Find a noded which can be shrunk.
	for _, nodedId := range nodeds {
		if _, _, ok := nodedOverprovisioned(m.sigmaFsl, m.ConfigClnt, m.realmId, nodedId, db.REALMMGR); ok {
			return nodedId, true
		}
	}
	return "", false
}

// Returns true if the realm should grow, and returns the queue length.
func (m *RealmResourceMgr) realmShouldGrow() (qlen int, hardReq bool, machineIds []string, shouldGrow bool) {
	lockRealm(m.lock, m.realmId)
	defer unlockRealm(m.lock, m.realmId)

	// Get the realm's config
	realmCfg, err := m.getRealmConfig()
	if err != nil {
		db.DPrintf(db.REALMMGR, "Error getRealmConfig: %v", err)
		return 0, false, machineIds, false
	}

	// If the realm is shutting down, return
	if realmCfg.Shutdown {
		return 0, false, machineIds, false
	}

	// If we don't have enough noded replicas to start the realm yet, we need to
	// grow the realm.
	if len(realmCfg.NodedsAssigned) < nReplicas() {
		return 1, true, machineIds, true
	}

	// If we haven't finished booting, we aren't ready to start scanning/growing
	// the realm.
	if len(realmCfg.NodedsActive) < nReplicas() {
		return 0, false, machineIds, false
	} else {
		// If the realm just finished booting, finish initialization.
		if m.FsLib == nil {
			m.FsLib = fslib.MakeFsLibAddr(proc.GetPid().String(), realmCfg.NamedAddrs)
		}
	}

	m.Lock()
	d := time.Since(m.lastGrow)
	m.Unlock()
	// If we have resized too recently, return
	if d < sp.Conf.Realm.RESIZE_INTERVAL {
		return 0, false, machineIds, false
	}

	// If there are a lot of procs waiting to be run/stolen...
	lcqlen, beqlen := m.getRealmQueueLen()
	qlen = lcqlen + beqlen
	if qlen >= int(machine.NodedNCores()) {
		// This is a hard reservation request (highest priority) if there are LC
		// procs queued.
		return qlen, lcqlen > 0, machineIds, true
	}
	var anyLC bool
	var utils map[string]float64
	var avgUtil float64
	avgUtil, utils, anyLC = m.getRealmUtil(realmCfg)
	db.DPrintf(db.REALMMGR, "[%v] Realm utils (avg:%v): %v", avgUtil, m.realmId, utils)
	// Filter machines to request more cores on by utilization, and sort in
	// order of importance.
	nodeds := sortNodedsByAscendingProcdUtil(utils)
	machineIds = make([]string, 0, len(nodeds))
	m.Lock()
	for i := len(nodeds) - 1; i >= 0; i-- {
		// Only grow allocations on highly-utilized machines.
		if utils[nodeds[i]] >= sp.Conf.Realm.GROW_CPU_UTIL_THRESHOLD {
			shouldGrow = true
			// Only request specific machines if there are LC procs running on them..
			if anyLC {
				machineIds = append(machineIds, m.nodedToMachined[nodeds[i]])
			}
		}
	}
	m.Unlock()
	// If no LC procs, and avg util is low, and no procs queued, don't grow.
	if !anyLC {
		shouldGrow = avgUtil >= sp.Conf.Realm.GROW_CPU_UTIL_THRESHOLD
	}
	return qlen, anyLC, machineIds, shouldGrow
}

func (m *RealmResourceMgr) Work() {
	db.DPrintf(db.REALMMGR, "Realmmgr started in realm %v", m.realmId)

	m.Started()
	go func() {
		m.WaitEvict(proc.GetPid())
		db.DPrintf(db.REALMMGR, "[%v] Evicted!", m.realmId)
		m.Exited(proc.MakeStatus(proc.StatusEvicted))
		db.DPrintf(db.REALMMGR, "[%v] Exited", m.realmId)
		os.Exit(0)
	}()

	for {
		qlen, hardReq, machineIds, ok := m.realmShouldGrow()
		if ok {
			db.DPrintf(db.REALMMGR, "[%v] Try to grow realm qlen %v", m.realmId, qlen)
			res := &proto.SigmaMgrResponse{}
			req := &proto.SigmaMgrRequest{
				RealmId:    m.realmId,
				Qlen:       int64(qlen),
				HardReq:    hardReq,
				MachineIds: machineIds,
			}
			err := m.sclnt.RPC("SigmaMgr.RequestCores", req, res)
			if err != nil || !res.OK {
				db.DFatalf("Error RPC: %v %v", err, res.OK)
			}
		}
		// Sleep for a bit.
		time.Sleep(sp.Conf.Realm.SCAN_INTERVAL)
	}
}
