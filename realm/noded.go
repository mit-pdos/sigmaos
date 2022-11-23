package realm

import (
	"path"
	"time"

	"sigmaos/config"
	db "sigmaos/debug"
	"sigmaos/electclnt"
	"sigmaos/fidclnt"
	"sigmaos/fslib"
	"sigmaos/kernel"
	"sigmaos/machine"
	"sigmaos/memfssrv"
	np "sigmaos/ninep"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/resource"
	"sigmaos/semclnt"
)

type Noded struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	*memfssrv.MemFs
	id        string
	machineId string
	localIP   string
	cfgPath   string
	done      chan bool
	cfg       *NodedConfig
	s         *kernel.System
	ec        *electclnt.ElectClnt
	*config.ConfigClnt
}

func MakeNoded(machineId string) *Noded {
	nd := &Noded{}
	nd.id = proc.GetPid().String()
	nd.machineId = machineId
	nd.cfgPath = NodedConfPath(nd.id)
	nd.done = make(chan bool)
	nd.FsLib = fslib.MakeFsLib(nd.id)
	nd.ProcClnt = procclnt.MakeProcClnt(nd.FsLib)
	nd.ConfigClnt = config.MakeConfigClnt(nd.FsLib)
	var err error
	nd.MemFs, err = memfssrv.MakeMemFsFsl(path.Join(machine.MACHINES, machineId, machine.NODEDS)+"/", nd.FsLib, nd.ProcClnt)
	if err != nil {
		db.DFatalf("Error MakeMemFsFsl: %v", err)
	}

	// Make a control file
	resource.MakeCtlFile(nd.receiveResourceGrant, nd.handleResourceRequest, nd.MemFs.Root(), np.RESOURCE_CTL)

	// Mount the KPIDS dir.
	if err := procclnt.MountPids(nd.FsLib, fslib.Named()); err != nil {
		db.DFatalf("Error mountpids: %v", err)
	}

	ip, err := fidclnt.LocalIP()
	if err != nil {
		db.DFatalf("Error LocalIP: %v", err)
	}
	nd.localIP = ip

	// Set the noded id so that child kernel procs inherit it.
	proc.SetNodedId(nd.id)

	// Set up the noded config
	nd.cfg = MakeNodedConfig()
	nd.cfg.Id = nd.id
	nd.cfg.RealmId = kernel.NO_REALM
	nd.cfg.MachineId = machineId

	// Write the initial config file
	nd.WriteConfig(nd.cfgPath, nd.cfg)

	return nd
}

func (nd *Noded) receiveResourceGrant(msg *resource.ResourceMsg) {
	switch msg.ResourceType {
	case resource.Tcore:
		db.DPrintf("NODED", "Noded %v granted cores %v", nd.id, msg.Name)
		nd.forwardResourceMsgToProcd(msg)

		cores := np.MkInterval(0, 0)
		cores.Unmarshal(msg.Name)

		nd.cfg.Cores = append(nd.cfg.Cores, cores)
		nd.WriteConfig(nd.cfgPath, nd.cfg)

		lockRealm(nd.ec, nd.cfg.RealmId)
		defer unlockRealm(nd.ec, nd.cfg.RealmId)

		// Update the realm's total core count. The Realmmgr holds the realm
		// lock.
		realmCfg := GetRealmConfig(nd.FsLib, nd.cfg.RealmId)
		realmCfg.NCores += proc.Tcore(cores.Size())
		nd.WriteConfig(RealmConfPath(nd.cfg.RealmId), realmCfg)

	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

func (nd *Noded) handleResourceRequest(msg *resource.ResourceMsg) {
	switch msg.ResourceType {
	case resource.Tcore:
		db.DPrintf("NODED", "Noded %v lost cores %v", nd.id, msg.Name)

		// If all cores were requested, shut down.
		if msg.Name == machine.ALL_CORES || len(nd.cfg.Cores) == 1 {
			// Leave the realm and prepare to shut down.
			nd.leaveRealm()
			nd.done <- true
			close(nd.done)
		} else {
			nd.forwardResourceMsgToProcd(msg)

			cores := nd.cfg.Cores[len(nd.cfg.Cores)-1]

			// Sanity check: should be at least 2 core groups when removing one.
			// Otherwise, we should have shut down.
			if len(nd.cfg.Cores) < 2 {
				db.DFatalf("Requesting cores form a noded with <2 core groups: %v", nd.cfg)
			}
			// Sanity check: we always take the last cores allocated.
			if cores.String() != msg.Name {
				db.DFatalf("Removed unexpected core group: %v from %v", msg.Name, nd.cfg)
			}

			// Update the core allocations for this noded.
			var rmCores *np.Tinterval
			nd.cfg.Cores, rmCores = nd.cfg.Cores[:len(nd.cfg.Cores)-1], nd.cfg.Cores[len(nd.cfg.Cores)-1]
			nd.WriteConfig(nd.cfgPath, nd.cfg)

			// Update the realm's total core count. The Realmmgr holds the realm
			// lock.
			realmCfg := GetRealmConfig(nd.FsLib, nd.cfg.RealmId)
			realmCfg.NCores -= proc.Tcore(rmCores.Size())
			nd.WriteConfig(RealmConfPath(nd.cfg.RealmId), realmCfg)

			machine.PostCores(nd.FsLib, nd.machineId, cores)
		}

	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

func (nd *Noded) forwardResourceMsgToProcd(msg *resource.ResourceMsg) {
	procdIp := nd.s.GetProcdIp()
	// Pass the resource message on to this noded's procd.
	resource.SendMsg(nd.FsLib, path.Join(RealmPath(nd.cfg.RealmId), np.PROCDREL, procdIp, np.RESOURCE_CTL), msg)
}

// Update configuration.
func (nd *Noded) getNextConfig() {
	for {
		nd.ReadConfig(nd.cfgPath, nd.cfg)
		// Make sure we've been assigned to a realm
		if nd.cfg.RealmId != kernel.NO_REALM {
			nd.ec = electclnt.MakeElectClnt(nd.FsLib, realmFencePath(nd.cfg.RealmId), 0777)
			break
		}
	}
}

func (nd *Noded) countNCores() proc.Tcore {
	ncores := proc.Tcore(0)
	for _, c := range nd.cfg.Cores {
		ncores += proc.Tcore(c.Size())
	}
	return ncores
}

// If we need more named replicas, help initialize a realm by starting another
// named replica for it. Return true when all named replicas have been
// initialized.
func (nd *Noded) tryAddNamedReplicaL() bool {
	// Get config
	realmCfg := GetRealmConfig(nd.FsLib, nd.cfg.RealmId)

	initDone := false
	// If this is the last required noded replica...
	if len(realmCfg.NodedsActive) == nReplicas()-1 {
		initDone = true
	}

	// If we need to add a named replica, do so
	if len(realmCfg.NodedsActive) < nReplicas() {
		namedAddrs := genNamedAddrs(1, nd.localIP)

		realmCfg.NamedAddrs = append(realmCfg.NamedAddrs, namedAddrs...)

		// Start a named instance.
		_, pid, err := kernel.BootNamed(nd.ProcClnt, namedAddrs[0], nReplicas() > 1, len(realmCfg.NamedAddrs), realmCfg.NamedAddrs, nd.cfg.RealmId)
		if err != nil {
			db.DFatalf("Error BootNamed in Noded.tryInitRealmL: %v", err)
		}
		// Update config
		realmCfg.NamedPids = append(realmCfg.NamedPids, pid.String())
		nd.WriteConfig(RealmConfPath(realmCfg.Rid), realmCfg)
		db.DPrintf("NODED", "Added named replica: %v", realmCfg)
	}
	return initDone
}

// Register this noded as part of a realm.
func (nd *Noded) register(cfg *RealmConfig) {
	cfg.NodedsActive = append(cfg.NodedsActive, nd.id)
	cfg.NCores += nd.countNCores()
	nd.WriteConfig(RealmConfPath(cfg.Rid), cfg)
	// Symlink into realmmgr's fs.
	if err := nd.Symlink(fslib.MakeTarget([]string{nd.MyAddr()}), nodedPath(cfg.Rid, nd.id), 0777); err != nil {
		db.DFatalf("Error symlink: %v", err)
	}
}

func (nd *Noded) boot(realmCfg *RealmConfig) {
	nd.s = kernel.MakeSystem("realm", realmCfg.Rid, realmCfg.NamedAddrs, nd.cfg.Cores[0])
	if err := nd.s.Boot(); err != nil {
		db.DFatalf("Error Boot in Noded.boot: %v", err)
	}
	// Update the config with the procd IP.
	nd.cfg.ProcdIp = nd.s.GetProcdIp()
	nd.WriteConfig(nd.cfgPath, nd.cfg)
}

// Join a realm
func (nd *Noded) joinRealm() {
	lockRealm(nd.ec, nd.cfg.RealmId)
	defer unlockRealm(nd.ec, nd.cfg.RealmId)

	// Try to initalize this realm if it hasn't been initialized already.
	initDone := nd.tryAddNamedReplicaL()
	// Get the realm config
	realmCfg := GetRealmConfig(nd.FsLib, nd.cfg.RealmId)
	// Register this noded
	nd.register(realmCfg)
	// Boot this noded's system services
	nd.boot(realmCfg)
	// Signal that the realm has been initialized
	if initDone {
		rStartSem := semclnt.MakeSemClnt(nd.FsLib, path.Join(np.BOOT, nd.cfg.RealmId))
		rStartSem.Up()
	}
	db.DPrintf("NODED", "Noded %v joined Realm %v", nd.id, nd.cfg.RealmId)
}

func (nd *Noded) teardown() {
	// Tear down realm resources
	nd.s.Shutdown()
}

func (nd *Noded) deregister(cfg *RealmConfig) {
	for i := range cfg.NodedsActive {
		if cfg.NodedsActive[i] == nd.id {
			cfg.NodedsActive = append(cfg.NodedsActive[:i], cfg.NodedsActive[i+1:]...)
			break
		}
	}

	for i := range cfg.NodedsAssigned {
		if cfg.NodedsAssigned[i] == nd.id {
			cfg.NodedsAssigned = append(cfg.NodedsAssigned[:i], cfg.NodedsAssigned[i+1:]...)
			break
		}
	}

	cfg.NCores -= nd.countNCores()
	cfg.LastResize = time.Now()

	nd.WriteConfig(RealmConfPath(cfg.Rid), cfg)

	// Remove the symlink to this noded from the realmmgr dir.
	nd.Remove(nodedPath(cfg.Rid, nd.id))

	for _, c := range nd.cfg.Cores {
		machine.PostCores(nd.FsLib, nd.machineId, c)
	}
}

func (nd *Noded) tryDestroyRealmL(realmCfg *RealmConfig) {
	// If this is the last noded, destroy the noded's named
	if len(realmCfg.NodedsActive) == 0 {
		db.DPrintf("NODED", "Destroy realm %v", realmCfg.Rid)

		ShutdownNamedReplicas(nd.ProcClnt, realmCfg.NamedPids)

		// Remove the realm config file
		if err := nd.Remove(RealmConfPath(realmCfg.Rid)); err != nil {
			db.DFatalf("Error Remove in REALM_CONFIG Noded.tryDestroyRealmL: %v", err)
		}

		// Remove the realm's named directory
		if err := nd.Remove(RealmPath(realmCfg.Rid)); err != nil {
			db.DPrintf("NODED_ERR", "Error Remove REALM_NAMEDS in Noded.tryDestroyRealmL: %v", err)
		}

		// Signal that the realm has been destroyed
		rExitSem := semclnt.MakeSemClnt(nd.FsLib, path.Join(np.BOOT, realmCfg.Rid))
		rExitSem.Up()
	}
}

// Leave a realm. Expects realmmgr to hold the realm lock.
func (nd *Noded) leaveRealm() {
	db.DPrintf("NODED", "Noded %v leaving Realm %v", nd.id, nd.cfg.RealmId)

	// Tear down resources
	nd.teardown()

	db.DPrintf("NODED", "Noded %v done with teardown", nd.id)

	// Get the realm config
	realmCfg := GetRealmConfig(nd.FsLib, nd.cfg.RealmId)
	// Deregister this noded
	nd.deregister(realmCfg)
	// Try to destroy a realm (if this is the last noded remaining)
	nd.tryDestroyRealmL(realmCfg)
}

func (nd *Noded) Work() {
	if err := nd.Started(); err != nil {
		db.DFatalf("Error Started: %v", err)
	}
	db.DPrintf("NODED", "Noded %v started", nd.id)
	// Get the next realm assignment.
	nd.getNextConfig()
	db.DPrintf("NODED", "Noded %v got config %v", nd.id, nd.cfg)

	// Join a realm
	nd.joinRealm()

	<-nd.done

	nd.Exited(proc.MakeStatus(proc.StatusOK))
}
