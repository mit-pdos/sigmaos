package realm

import (
	"path"
	"time"

	db "ulambda/debug"
	"ulambda/electclnt"
	"ulambda/machine"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/resource"
)

type Manager interface {
	getRealmElectClnt(string) (*electclnt.ElectClnt, bool)
	// config.ConfigClnt
	ReadConfig(string, interface{})
	WriteConfig(string, interface{})
	// fslib.FsLib
	SetFile(fname string, data []byte, m np.Tmode, off np.Toffset) (np.Tsize, error)
}

// Request a machine to create a new Noded)
func requestNoded(m Manager, machineId string) proc.Tpid {
	pid := proc.Tpid("noded-" + proc.GenPid().String())
	msg := resource.MakeResourceMsg(resource.Trequest, resource.Tnode, pid.String(), 1)
	if _, err := m.SetFile(path.Join(machine.MACHINES, machineId, np.RESOURCE_CTL), msg.Marshal(), np.OWRITE, 0); err != nil {
		db.DFatalf("Error SetFile in requestNoded: %v", err)
	}
	return pid
}

// Alloc a Noded to this realm.
func allocNoded(m Manager, realmId string, nodedId string) {
	// If the realm has been destroyed, exit early.
	ec, ok := m.getRealmElectClnt(realmId)
	if !ok {
		return
	}

	lockRealm(ec, realmId)
	defer unlockRealm(ec, realmId)

	// Update the noded's config
	ndCfg := &NodedConfig{}
	ndCfg.Id = nodedId
	ndCfg.RealmId = realmId
	m.WriteConfig(path.Join(NODED_CONFIG, nodedId), ndCfg)

	// Update the realm's config
	rCfg := &RealmConfig{}
	m.ReadConfig(path.Join(REALM_CONFIG, realmId), rCfg)
	rCfg.NodedsAssigned = append(rCfg.NodedsAssigned, nodedId)
	rCfg.LastResize = time.Now()
	m.WriteConfig(path.Join(REALM_CONFIG, realmId), rCfg)
}
