package mntclnt

import (
	"fmt"

	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

type NamedEndpointCache struct {
	nces *syncmap.SyncMap[sp.Trealm, *sp.Tendpoint]
}

func newNamedEndpointCache(pe *proc.ProcEnv) *NamedEndpointCache {
	nec := &NamedEndpointCache{
		nces: syncmap.NewSyncMap[sp.Trealm, *sp.Tendpoint](),
	}
	// If an initial named mount was provided to this proc, set it.
	if ep, ok := pe.GetNamedEndpoint(); ok {
		nec.nces.Insert(pe.GetRealm(), ep)
	}
	return nec
}

func (nec *NamedEndpointCache) String() string {
	return fmt.Sprintf("{nmc %v}", nec.nces)
}

func (nec *NamedEndpointCache) Get(realm sp.Trealm) (*sp.Tendpoint, bool) {
	return nec.nces.Lookup(realm)
}

func (nec *NamedEndpointCache) Put(realm sp.Trealm, ep *sp.Tendpoint) {
	nec.nces.Insert(realm, ep)
}

func (nec *NamedEndpointCache) Invalidate(realm sp.Trealm) {
	nec.nces.Delete(realm)
}
