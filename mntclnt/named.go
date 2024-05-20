package mntclnt

import (
	"fmt"
	gpath "path"
	"time"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	path "sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func (mc *MntClnt) GetNamedEndpointRealm(realm sp.Trealm) (*sp.Tendpoint, error) {
	if ep, err := mc.getNamedEndpointRealm(realm); err != nil {
		return ep, err
	} else {
		return ep, nil
	}
}

func (mc *MntClnt) InvalidateNamedEndpointCacheEntryRealm(realm sp.Trealm) error {
	return mc.invalidateNamedMountCacheEntry(realm)
}

// Get named enpoint via netproxy or directly
func (mc *MntClnt) getNamedEndpointRealm(realm sp.Trealm) (*sp.Tendpoint, *serr.Err) {
	s := time.Now()
	if ep, ok := mc.ndMntCache.Get(realm); ok {
		db.DPrintf(db.MOUNT, "getNamedEndpointRealm cached %v %v", realm, ep)
		return ep, nil
	}
	var ep *sp.Tendpoint
	if mc.pe.UseNetProxy {
		// If this mount was passed via the proc env, return it immediately.
		ep0, err := mc.npc.GetNamedEndpoint(realm)
		if err != nil {
			if sr, ok := serr.IsErr(err); ok {
				return &sp.Tendpoint{}, sr
			}
			return &sp.Tendpoint{}, serr.NewErrError(err)
		}
		ep = ep0
	} else {
		ep0, err := mc.getNamedEndpointDirect(realm)
		if err != nil {
			return &sp.Tendpoint{}, err
		}
		ep = ep0
	}
	// Cache the newly resolved mount
	mc.ndMntCache.Put(realm, ep)
	db.DPrintf(db.MOUNT, "getNamedEndpointRealm [%v] %v", realm, ep)
	db.DPrintf(db.WALK_LAT, "getNamedEndpointRealm %v %v %v", mc.cid, realm, time.Since(s))
	return ep, nil

}

// Get named enpoint directly
func (mc *MntClnt) getNamedEndpointDirect(realm sp.Trealm) (*sp.Tendpoint, *serr.Err) {
	// If this is the root realm, then get the root named.
	if realm == sp.ROOTREALM {
		s := time.Now()
		ep, err := fsetcd.GetRootNamed(mc.npc.Dial, mc.pe.GetEtcdEndpoints(), realm)
		if err != nil {
			db.DPrintf(db.MOUNT_ERR, "getNamedEndpointDirect [%v] err GetRootNamed %v", realm, ep)
			if sr, ok := serr.IsErr(err); ok {
				return &sp.Tendpoint{}, sr
			}
			return &sp.Tendpoint{}, serr.NewErrError(err)
		}
		db.DPrintf(db.WALK_LAT, "getNamedEndpointDirect %v %v %v", mc.cid, sp.ROOTREALM, time.Since(s))
		return ep, nil
	} else {
		// Otherwise, walk through the root named to find this named's mount.
		s := time.Now()
		if _, rest, err := mc.mnt.resolveMnt(path.Path{"root"}, true); err != nil && len(rest) >= 1 {
			if err := mc.mountNamed(sp.ROOTREALM, "root"); err != nil {
				db.DPrintf(db.MOUNT_ERR, "getNamedEndpointDirect [%v] err mounting root named %v", realm, err)
				return &sp.Tendpoint{}, err
			}
		}
		db.DPrintf(db.WALK_LAT, "getNamedEndpointDirect %v mount %v %v", mc.cid, sp.ROOTREALM, time.Since(s))
		s = time.Now()
		pn := gpath.Join("root", sp.REALMREL, sp.REALMDREL, sp.REALMSREL, realm.String())
		target, err := mc.pathc.GetFile(pn, mc.pe.GetPrincipal(), sp.OREAD, 0, sp.MAXGETSET, sp.NullFence())
		if err != nil {
			db.DPrintf(db.MOUNT_ERR, "getNamedEndpointDirect [%v] GetFile err %v", realm, err)
			return &sp.Tendpoint{}, serr.NewErrError(err)
		}
		ep, err := sp.NewEndpointFromBytes(target)
		if err != nil {
			return &sp.Tendpoint{}, serr.NewErrError(err)
		}
		db.DPrintf(db.WALK_LAT, "getNamedEndpointDirect %v getfile %v %v", mc.cid, realm, time.Since(s))
		return ep, nil
	}
}

func (mc *MntClnt) invalidateNamedMountCacheEntry(realm sp.Trealm) error {
	mc.ndMntCache.Invalidate(realm)
	if mc.pe.UseNetProxy {
		return mc.npc.InvalidateNamedEndpointCacheEntry(realm)
	}
	return nil
}

func (mc *MntClnt) mountNamed(realm sp.Trealm, name string) *serr.Err {
	s := time.Now()
	ep, err := mc.getNamedEndpointRealm(realm)
	if err != nil {
		db.DPrintf(db.MOUNT_ERR, "mountNamed [%v]: getNamedMount err %v", realm, err)
		return err
	}
	if err := mc.AutoMount(mc.pe.GetSecrets(), ep, path.Path{name}); err != nil {
		db.DPrintf(db.MOUNT_ERR, "mountNamed: automount err %v", err)
		// If mounting failed, the named is unreachable. Invalidate the cache entry
		// for this realm.
		if err := mc.invalidateNamedMountCacheEntry(realm); err != nil {
			db.DPrintf(db.ERROR, "Error invalidating named mount cache entry: %v", err)
		}
		return serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v realm failure", realm))
	}
	db.DPrintf(db.MOUNT, "mountNamed [%v]: automount ep %v at %v", realm, ep, name)
	db.DPrintf(db.WALK_LAT, "mountNamed [%v]: %v automount ep %v at %v lat %v", mc.cid, realm, ep, name, time.Since(s))
	return nil
}
