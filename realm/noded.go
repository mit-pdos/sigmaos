package realm

import (
	"fmt"
	"path"
	"strings"

	"ulambda/config"
	db "ulambda/debug"
	"ulambda/electclnt"
	"ulambda/fidclnt"
	"ulambda/fslib"
	"ulambda/kernel"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/resource"
	"ulambda/semclnt"
)

type NodedConfig struct {
	Id      string
	RealmId string
}

type Noded struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	id      string
	bin     string
	cfgPath string
	cfg     *NodedConfig
	s       *kernel.System
	ec      *electclnt.ElectClnt
	*config.ConfigClnt
}

func MakeNoded(bin string, id string) *Noded {
	r := &Noded{}
	r.id = id
	r.bin = bin
	r.cfgPath = path.Join(NODED_CONFIG, id)
	r.FsLib = fslib.MakeFsLib(fmt.Sprintf("noded-%v", id))
	r.ProcClnt = procclnt.MakeProcClntInit(proc.Tpid("noded-"+id), r.FsLib, "noded", fslib.Named())
	r.ConfigClnt = config.MakeConfigClnt(r.FsLib)

	db.DPrintf("NODED", "Noded %v started", id)

	// Set the noded id so that child kernel procs inherit it.
	proc.SetNodedId(id)

	// Set up the noded config
	r.cfg = &NodedConfig{}
	r.cfg.Id = id
	r.cfg.RealmId = kernel.NO_REALM

	// Write the initial config file
	r.WriteConfig(r.cfgPath, r.cfg)

	// Mark self as available for allocation
	r.markFree()

	return r
}

// Mark self as available for allocation to a realm.
func (r *Noded) markFree() {
	cfg := &NodedConfig{}
	cfg.Id = r.id
	cfg.RealmId = kernel.NO_REALM

	msg := resource.MakeResourceMsg(resource.Tgrant, resource.Tnode, r.id, 1)

	if _, err := r.SetFile(SIGMACTL, msg.Marshal(), np.OWRITE, 0); err != nil {
		db.DFatalf("Error SetFile in markFree: %v", err)
	}
}

// Update configuration.
func (r *Noded) getNextConfig() {
	// XXX Does it matter that we spin?
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
		ip, err := fidclnt.LocalIP()
		if err != nil {
			db.DFatalf("Error LocalIP in Noded.tryInitRealmL: %v", err)
		}
		namedAddrs := genNamedAddrs(1, ip)

		realmCfg.NamedAddrs = append(realmCfg.NamedAddrs, namedAddrs...)

		// Start a named instance.
		var pid proc.Tpid
		if _, pid, err = kernel.BootNamed(r.ProcClnt, r.bin, namedAddrs[0], nReplicas() > 1, len(realmCfg.NamedAddrs), realmCfg.NamedAddrs, r.cfg.RealmId); err != nil {
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
func (r *Noded) register(cfg *RealmConfig) {
	cfg.NodedsActive = append(cfg.NodedsActive, r.id)
	r.WriteConfig(path.Join(REALM_CONFIG, cfg.Rid), cfg)
}

func (r *Noded) boot(realmCfg *RealmConfig) {
	r.s = kernel.MakeSystem("realm", r.bin, realmCfg.NamedAddrs)
	if err := r.s.Boot(); err != nil {
		db.DFatalf("Error Boot in Noded.boot: %v", err)
	}
}

func (r *Noded) startRealmMgr() {
	realmCfg := GetRealmConfig(r.FsLib, r.cfg.RealmId)
	pid := proc.Tpid("realmmgr-" + proc.GenPid().String())
	p := proc.MakeProcPid(pid, "bin/realm/realmmgr", []string{r.cfg.RealmId, strings.Join(realmCfg.NamedAddrs, ",")})
	if _, err := r.SpawnKernelProc(p, r.bin, fslib.Named()); err != nil {
		db.DFatalf("Error spawn realmmgr %v", err)
	}
	if err := r.WaitStart(p.Pid); err != nil {
		db.DFatalf("Error WaitStart realmmgr %v", err)
	}
	db.DPrintf("NODED", "Noded %v started its realmmgr %v in realm %v", r.id, pid.String(), r.cfg.RealmId)
}

// Join a realm
func (r *Noded) joinRealm() chan bool {
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
	// Watch for changes to the config
	return r.WatchConfig(r.cfgPath)
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

func (r *Noded) Work() {
	for {
		// Get the next realm assignment.
		r.getNextConfig()

		// Join a realm
		done := r.joinRealm()
		r.startRealmMgr()
		// Wait for the watch to trigger
		<-done

		// Leave a realm
		r.leaveRealm()

		// Mark self as available for allocation.
		r.markFree()
	}
}
