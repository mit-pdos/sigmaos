package realm

import (
	"fmt"
	"time"

	"ulambda/config"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
)

type NodedConfig struct {
	Id        string
	MachineId string
	RealmId   string
	ProcdIp   string
	Cores     []*np.Tinterval
}

func MakeNodedConfig() *NodedConfig {
	cfg := &NodedConfig{}
	cfg.Cores = []*np.Tinterval{}
	return cfg
}

func (cfg *NodedConfig) String() string {
	return fmt.Sprintf("&{ id:%v machineId:%v realmId:%v cores:%v }", cfg.Id, cfg.MachineId, cfg.RealmId, cfg.Cores)
}

type RealmConfig struct {
	Rid            string     // Realm id.
	NodedsAssigned []string   // Slice of noded IDs which have been assigned to this realm.
	NodedsActive   []string   // Slice of noded IDs which are currently actively part of this realm.
	LastResize     time.Time  // Timestamp from the last time this realm was resized
	Shutdown       bool       // True if this realm is in the process of being destroyed.
	NamedAddrs     []string   // IP addresses of this realm's nameds.
	NamedPids      []string   // PIDs of this realm's nameds.
	NCores         proc.Tcore // Number of cores assigned to this realm.
}

func (rc *RealmConfig) String() string {
	return fmt.Sprintf("&{ rid:%v mdAssigned:%v mdActive:%v lastResize:%v shutdown:%v namedAddrs:%v namedPids:%v nCores:%v }", rc.Rid, rc.NodedsAssigned, rc.NodedsActive, rc.LastResize, rc.Shutdown, rc.NamedAddrs, rc.NamedPids, rc.NCores)
}

// Get a realm's configuration
func GetRealmConfig(fsl *fslib.FsLib, rid string) *RealmConfig {
	clnt := config.MakeConfigClnt(fsl)
	cfg := &RealmConfig{}
	clnt.ReadConfig(RealmConfPath(rid), cfg)
	return cfg
}
