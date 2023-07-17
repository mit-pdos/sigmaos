package ephemeralmap

import (
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

type EphemeralMap struct {
	*syncmap.SyncMap[string, sp.TleaseId]
}

func NewEphemeralMap() *EphemeralMap {
	et := &EphemeralMap{SyncMap: syncmap.NewSyncMap[string, sp.TleaseId]()}
	return et
}
