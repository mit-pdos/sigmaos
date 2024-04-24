package pathclnt

import (
	"sync"

	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type NamedMountCache struct {
	sync.RWMutex
	root  *sp.Tendpoint
	realm *sp.Tendpoint
}

func NewNamedMountCache(pe *proc.ProcEnv) *NamedMountCache {
	var rootMnt *sp.Tendpoint = nil
	var realmMnt *sp.Tendpoint = nil
	// If an initial named mount was provided to this proc, set it.
	if mnt, ok := pe.GetNamedEndpoint(); ok {
		// If this proc operates in the root realm, cache the root mount as well
		if pe.GetRealm() == sp.ROOTREALM {
			rootMnt = mnt
		} else {
			realmMnt = mnt
		}
	}
	return &NamedMountCache{
		root:  rootMnt,
		realm: realmMnt,
	}
}

func (nmc *NamedMountCache) Get(realm sp.Trealm) (*sp.Tendpoint, bool) {
	nmc.RLock()
	defer nmc.RUnlock()

	if realm == sp.ROOTREALM {
		if nmc.root == nil {
			return &sp.Tendpoint{}, false
		}
		return nmc.root, true
	}
	if nmc.realm == nil {
		return &sp.Tendpoint{}, false
	}
	return nmc.realm, true
}

func (nmc *NamedMountCache) Put(realm sp.Trealm, mnt *sp.Tendpoint) {
	nmc.Lock()
	defer nmc.Unlock()

	if realm == sp.ROOTREALM {
		nmc.root = mnt
	} else {
		nmc.realm = mnt
	}
}

func (nmc *NamedMountCache) Invalidate(realm sp.Trealm) {
	nmc.Lock()
	defer nmc.Unlock()

	if realm == sp.ROOTREALM {
		nmc.root = nil
	} else {
		nmc.realm = nil
	}
}
