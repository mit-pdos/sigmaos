package realm

import (
	"fmt"
	"time"
)

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
