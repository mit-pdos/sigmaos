package realm

import (
	"fmt"
	"path"
	"time"

	"ulambda/config"
	"ulambda/fslib"
	np "ulambda/ninep"
)

type NodedConfig struct {
	Id        string
	MachineId string
	RealmId   string
	Cores     *np.Tinterval
}

func (cfg *NodedConfig) String() string {
	return fmt.Sprintf("&{ id:%v machineId:%v realmId:%v cores:%v }", cfg.Id, cfg.MachineId, cfg.RealmId, cfg.Cores)
}

type RealmConfig struct {
	Rid            string    // Realm id.
	NodedsAssigned []string  // Slice of noded IDs which have been assigned to this realm.
	NodedsActive   []string  // Slice of noded IDs which are currently actively part of this realm.
	LastResize     time.Time // Timestamp from the last time this realm was resized
	Shutdown       bool      // True if this realm is in the process of being destroyed.
	NamedAddrs     []string  // IP addresses of this realm's nameds.
	NamedPids      []string  // PIDs of this realm's nameds.
}

func (rc *RealmConfig) String() string {
	return fmt.Sprintf("&{ rid:%v mdAssigned:%v mdActive:%v lastResize:%v shutdown:%v namedAddrs:%v namedPids:%v }", rc.Rid, rc.NodedsAssigned, rc.NodedsActive, rc.LastResize, rc.Shutdown, rc.NamedAddrs, rc.NamedPids)
}

// Get a realm's configuration
func GetRealmConfig(fsl *fslib.FsLib, rid string) *RealmConfig {
	clnt := config.MakeConfigClnt(fsl)
	cfg := &RealmConfig{}
	clnt.ReadConfig(path.Join(REALM_CONFIG, rid), cfg)
	return cfg
}
