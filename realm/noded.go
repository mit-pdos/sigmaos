package realm

import (
	"path"

	"ulambda/config"
	db "ulambda/debug"
	"ulambda/electclnt"
	"ulambda/fidclnt"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/kernel"
	"ulambda/machine"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/resource"
	"ulambda/semclnt"
)

type Noded struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	*fslibsrv.MemFs
	id        string
	machineId string
	localIP   string
	cfgPath   string
	cfg       *NodedConfig
	s         *kernel.System
	ec        *electclnt.ElectClnt
	*config.ConfigClnt
}

func MakeNoded(machineId string) *Noded {
	r := &Noded{}
	r.id = proc.GetPid().String()
	r.machineId = machineId
	r.cfgPath = path.Join(NODED_CONFIG, r.id)
	r.FsLib = fslib.MakeFsLib(r.id)
	r.ProcClnt = procclnt.MakeProcClnt(r.FsLib)
	r.ConfigClnt = config.MakeConfigClnt(r.FsLib)
	var err error
	r.MemFs, err = fslibsrv.MakeMemFsFsl(path.Join(machine.MACHINES, machineId, machine.NODEDS)+"/", r.FsLib, r.ProcClnt)
	if err != nil {
		db.DFatalf("Error MakeMemFsFsl: %v", err)
	}

	// Make a control file
	resource.MakeCtlFile(r.receiveResourceGrant, r.handleResourceRequest, r.MemFs.Root(), np.RESOURCE_CTL)

	// Mount the KPIDS dir.
	if err := procclnt.MountPids(r.FsLib, fslib.Named()); err != nil {
		db.DFatalf("Error mountpids: %v", err)
	}

	ip, err := fidclnt.LocalIP()
	if err != nil {
		db.DFatalf("Error LocalIP: %v", err)
	}
	r.localIP = ip

	// Set the noded id so that child kernel procs inherit it.
	proc.SetNodedId(r.id)

	// Set up the noded config
	r.cfg = MakeNodedConfig()
	r.cfg.Id = r.id
	r.cfg.RealmId = kernel.NO_REALM
	r.cfg.MachineId = machineId

	// Write the initial config file
	r.WriteConfig(r.cfgPath, r.cfg)

	return r
}

func (nd *Noded) receiveResourceGrant(msg *resource.ResourceMsg) {
	switch msg.ResourceType {
	case resource.Tcore:
		db.DPrintf("NODED", "Noded %v granted cores %v", nd.id, msg.Name)
		nd.forwardResourceMsgToProcd(msg)

		cores := np.MkInterval(0, 0)
		cores.Unmarshal(msg.Name)

		// Update the noded config file.
		nd.ReadConfig(nd.cfgPath, nd.cfg)
		nd.cfg.Cores = append(nd.cfg.Cores, cores)
		nd.WriteConfig(nd.cfgPath, nd.cfg)

	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

func (nd *Noded) handleResourceRequest(msg *resource.ResourceMsg) {
	switch msg.ResourceType {
	case resource.Tcore:
		db.DPrintf("NODED", "Noded %v lost cores %v", nd.id, msg.Name)
		nd.forwardResourceMsgToProcd(msg)

		// Update the noded config file.
		nd.ReadConfig(nd.cfgPath, nd.cfg)

		cores := nd.cfg.Cores[len(nd.cfg.Cores)-1]

		// XXX maybe remove these sanity checks...
		// Sanity check: should be at least 2 core groups when removing one.
		if len(nd.cfg.Cores) < 2 {
			db.DFatalf("Requesting cores form a noded with <2 core groups: %v", nd.cfg)
		}
		// Sanity check: we always take the last cores allocated.
		if cores.String() != msg.Name {
			db.DFatalf("Removed unexpected core group: %v from %v", msg.Name, nd.cfg)
		}

		// Update the core allocations for this noded.
		nd.cfg.Cores = nd.cfg.Cores[:len(nd.cfg.Cores)-1]
		nd.WriteConfig(nd.cfgPath, nd.cfg)

		machine.PostCores(nd.FsLib, nd.machineId, cores)

	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

func (nd *Noded) forwardResourceMsgToProcd(msg *resource.ResourceMsg) {
	procdIp := nd.s.GetProcdIp()
	// Pass the resource message on to this noded's procd.
	if _, err := nd.SetFile(path.Join(REALM_NAMEDS, nd.cfg.RealmId, np.PROCDREL, procdIp, np.RESOURCE_CTL), msg.Marshal(), np.OWRITE, 0); err != nil {
		db.DFatalf("Error SetFile: %v", err)
	}
}

// Update configuration.
func (r *Noded) getNextConfig() {
	for {
		r.ReadConfig(r.cfgPath, r.cfg)
		// Make sure we've been assigned to a realm
		if r.cfg.RealmId != kernel.NO_REALM {
			r.ec = electclnt.MakeElectClnt(r.FsLib, path.Join(REALM_FENCES, r.cfg.RealmId), 0777)
			break
		}
	}
}

// If we need more named replicas, help initialize a realm by starting another
// named replica for it. Return true when all named replicas have been
// initialized.
func (r *Noded) tryAddNamedReplicaL() bool {
	// Get config
	realmCfg := GetRealmConfig(r.FsLib, r.cfg.RealmId)

	initDone := false
	// If this is the last required noded replica...
	if len(realmCfg.NodedsActive) == nReplicas()-1 {
		initDone = true
	}

	// If we need to add a named replica, do so
	if len(realmCfg.NodedsActive) < nReplicas() {
		namedAddrs := genNamedAddrs(1, r.localIP)

		realmCfg.NamedAddrs = append(realmCfg.NamedAddrs, namedAddrs...)

		// Start a named instance.
		_, pid, err := kernel.BootNamed(r.ProcClnt, namedAddrs[0], nReplicas() > 1, len(realmCfg.NamedAddrs), realmCfg.NamedAddrs, r.cfg.RealmId)
		if err != nil {
			db.DFatalf("Error BootNamed in Noded.tryInitRealmL: %v", err)
		}
		// Update config
		realmCfg.NamedPids = append(realmCfg.NamedPids, pid.String())
		r.WriteConfig(path.Join(REALM_CONFIG, realmCfg.Rid), realmCfg)
		db.DPrintf("NODED", "Added named replica: %v", realmCfg)
	}
	return initDone
}

// Register this noded as part of a realm.
func (nd *Noded) register(cfg *RealmConfig) {
	cfg.NodedsActive = append(cfg.NodedsActive, nd.id)
	nd.WriteConfig(path.Join(REALM_CONFIG, cfg.Rid), cfg)
	// Symlink into realmmgr's fs.
	if err := nd.Symlink(fslib.MakeTarget([]string{nd.MyAddr()}), path.Join(REALM_MGRS, cfg.Rid, NODEDS, nd.id), 0777); err != nil {
		db.DFatalf("Error symlink: %v", err)
	}
}

func (r *Noded) boot(realmCfg *RealmConfig) {
	r.s = kernel.MakeSystem("realm", realmCfg.Rid, realmCfg.NamedAddrs, r.cfg.Cores[0])
	if err := r.s.Boot(); err != nil {
		db.DFatalf("Error Boot in Noded.boot: %v", err)
	}
}

// Join a realm
func (r *Noded) joinRealm() {
	lockRealm(r.ec, r.cfg.RealmId)
	defer unlockRealm(r.ec, r.cfg.RealmId)

	// Try to initalize this realm if it hasn't been initialized already.
	initDone := r.tryAddNamedReplicaL()
	// Get the realm config
	realmCfg := GetRealmConfig(r.FsLib, r.cfg.RealmId)
	// Register this noded
	r.register(realmCfg)
	// Boot this noded's system services
	r.boot(realmCfg)
	// Signal that the realm has been initialized
	if initDone {
		rStartSem := semclnt.MakeSemClnt(r.FsLib, path.Join(np.BOOT, r.cfg.RealmId))
		rStartSem.Up()
	}
	db.DPrintf("NODED", "Noded %v joined Realm %v", r.id, r.cfg.RealmId)
}

func (r *Noded) teardown() {
	// Tear down realm resources
	r.s.Shutdown()
}

func (r *Noded) deregister(cfg *RealmConfig) {
	for i := range cfg.NodedsActive {
		if cfg.NodedsActive[i] == r.id {
			cfg.NodedsActive = append(cfg.NodedsActive[:i], cfg.NodedsActive[i+1:]...)
			break
		}
	}
	r.WriteConfig(path.Join(REALM_CONFIG, cfg.Rid), cfg)

	for _, c := range r.cfg.Cores {
		machine.PostCores(r.FsLib, r.machineId, c)
	}
}

func (r *Noded) tryDestroyRealmL(realmCfg *RealmConfig) {
	// If this is the last noded, destroy the noded's named
	if len(realmCfg.NodedsActive) == 0 {
		ShutdownNamedReplicas(r.ProcClnt, realmCfg.NamedPids)

		// Remove the realm config file
		if err := r.Remove(path.Join(REALM_CONFIG, r.cfg.RealmId)); err != nil {
			db.DFatalf("Error Remove in REALM_CONFIG Noded.tryDestroyRealmL: %v", err)
		}

		// Remove the realm's named directory
		if err := r.Remove(path.Join(REALM_NAMEDS, r.cfg.RealmId)); err != nil {
			db.DFatalf("Error Remove REALM_NAMEDS in Noded.tryDestroyRealmL: %v", err)
		}

		// Signal that the realm has been destroyed
		rExitSem := semclnt.MakeSemClnt(r.FsLib, path.Join(np.BOOT, r.cfg.RealmId))
		rExitSem.Up()
	}
}

// Leave a realm
func (r *Noded) leaveRealm() {
	db.DPrintf("NODED", "Noded %v trying to leave Realm %v", r.id, r.cfg.RealmId)

	lockRealm(r.ec, r.cfg.RealmId)
	defer unlockRealm(r.ec, r.cfg.RealmId)

	db.DPrintf("NODED", "Noded %v leaving Realm %v", r.id, r.cfg.RealmId)

	// Tear down resources
	r.teardown()

	db.DPrintf("NODED", "Noded %v done with teardown", r.id)

	// Get the realm config
	realmCfg := GetRealmConfig(r.FsLib, r.cfg.RealmId)
	// Deregister this noded
	r.deregister(realmCfg)
	// Try to destroy a realm (if this is the last noded remaining)
	r.tryDestroyRealmL(realmCfg)
}

// Wait until we are deallocated from the realm.
func (nd *Noded) waitForDealloc() {
	for {
		// Watch for changes to the config
		done := nd.WatchConfig(nd.cfgPath)
		<-done
		nd.ReadConfig(nd.cfgPath, nd.cfg)
		// Make sure we've been assigned to a realm
		if nd.cfg.RealmId == kernel.NO_REALM {
			break
		}
	}
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

	nd.waitForDealloc()

	// Leave a realm
	nd.leaveRealm()
	nd.Exited(proc.MakeStatus(proc.StatusOK))
}
