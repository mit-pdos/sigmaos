package realm

import (
	"time"
)

type RealmConfig struct {
	Rid               string    // Realm id.
	MachinedsAssigned []string  // Slice of machined IDs which have been assigned to this realm.
	MachinedsActive   []string  // Slice of machined IDs which are currently actively part of this realm.
	LastResize        time.Time // Timestamp from the last time this realm was resized
	Shutdown          bool      // True if this realm is in the process of being destroyed.
	NamedAddrs        []string  // IP addresses of this realm's nameds.
	NamedPids         []string  // PIDs of this realm's nameds.
}
