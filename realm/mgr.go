package realm

import (
	"path"
	"time"

	"ulambda/electclnt"
	np "ulambda/ninep"
)

type Manager interface {
	// config.ConfigClnt
	ReadConfig(string, interface{})
	WriteConfig(string, interface{})
	SetFile(fname string, data []byte, m np.Tmode, off np.Toffset) (np.Tsize, error)
}

// Alloc a Noded to this realm.
func allocNoded(m Manager, ec *electclnt.ElectClnt, realmId, nodedId string, cores *np.Tinterval) {
	// Update the noded's config
	ndCfg := &NodedConfig{}
	ndCfg.Id = nodedId
	ndCfg.RealmId = realmId
	ndCfg.Cores = cores
	m.WriteConfig(path.Join(NODED_CONFIG, nodedId), ndCfg)

	lockRealm(ec, realmId)
	defer unlockRealm(ec, realmId)

	// Update the realm's config
	rCfg := &RealmConfig{}
	m.ReadConfig(path.Join(REALM_CONFIG, realmId), rCfg)
	rCfg.NodedsAssigned = append(rCfg.NodedsAssigned, nodedId)
	rCfg.LastResize = time.Now()
	m.WriteConfig(path.Join(REALM_CONFIG, realmId), rCfg)
}
