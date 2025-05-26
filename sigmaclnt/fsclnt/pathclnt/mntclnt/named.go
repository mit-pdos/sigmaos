package mntclnt

import (
	"fmt"
	"path/filepath"
	"time"

	db "sigmaos/debug"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/path"
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

// Get named enpoint via dialproxy or directly
func (mc *MntClnt) getNamedEndpointRealm(realm sp.Trealm) (*sp.Tendpoint, *serr.Err) {
	s := time.Now()
	if ep, ok := mc.ndMntCache.Get(realm); ok {
		db.DPrintf(db.MOUNT, "getNamedEndpointRealm cached %v %v", realm, ep)
		return ep, nil
	}
	var ep *sp.Tendpoint
	if mc.pe.UseDialProxy {
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
	db.DPrintf(db.MOUNT, "getNamedEndpointRealm %v [%v] %v", mc.cid, realm, ep)
	db.DPrintf(db.WALK_LAT, "getNamedEndpointRealm %v %v %v", mc.cid, ep, time.Since(s))
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
		if _, rest, err := mc.ResolveMnt(path.Tpathname{sp.ROOT, sp.REALMSREL}, true); err != nil && len(rest) >= 1 {
			// Mount the realm dir from the root named
			if err := mc.mountNamed(sp.ROOTREALM, filepath.Join(sp.ROOT, sp.REALMSREL), sp.REALMSREL); err != nil {
				db.DPrintf(db.MOUNT_ERR, "getNamedEndpointDirect [%v] err mounting root named %v", realm, err)
				return &sp.Tendpoint{}, err
			}
		}
		db.DPrintf(db.WALK_LAT, "getNamedEndpointDirect %v mount %v %v", mc.cid, sp.ROOTREALM, time.Since(s))
		s = time.Now()
		pn := filepath.Join(sp.ROOT, sp.REALMSREL, realm.String())
		target, err := mc.pathc.GetFile(pn, mc.pe.GetPrincipal(), sp.OREAD, 0, sp.MAXGETSET, sp.NullFence())
		if err != nil {
			db.DPrintf(db.MOUNT_ERR, "getNamedEndpointDirect [%v] GetFile err %v", realm, err)
			if sr, ok := serr.IsErr(err); ok {
				return &sp.Tendpoint{}, sr
			}
			return &sp.Tendpoint{}, serr.NewErrError(err)
		}
		ep, err := sp.NewEndpointFromBytes(target)
		if err != nil {
			return &sp.Tendpoint{}, serr.NewErrError(err)
		}
		db.DPrintf(db.WALK_LAT, "getNamedEndpointDirect %v getfile %v %v", mc.cid, ep, time.Since(s))
		return ep, nil
	}
}

func (mc *MntClnt) invalidateNamedMountCacheEntry(realm sp.Trealm) error {
	db.DPrintf(db.NAMED_LDR, "invalidateNamedMountCacheEntry %v", realm)
	if realm != sp.ROOTREALM {
		pn := filepath.Join(sp.ROOT, sp.REALMSREL, realm.String())
		mc.mnt.umount(path.Split(pn), true)
	}
	mc.ndMntCache.Invalidate(realm)
	if mc.pe.UseDialProxy {
		return mc.npc.InvalidateNamedEndpointCacheEntry(realm)
	}
	return nil
}

func (mc *MntClnt) mountNamed(realm sp.Trealm, mntName, tree sp.Tsigmapath) *serr.Err {
	db.DPrintf(db.MOUNT, "mountNamed [%v] at %v tree \"%v\"", realm, mntName, tree)
	s := time.Now()
	ep, err := mc.getNamedEndpointRealm(realm)
	if err != nil {
		db.DPrintf(db.MOUNT_ERR, "mountNamed [%v]: getNamedMount err %v", realm, err)
		return err
	}
	if err := mc.MountTree(mc.pe.GetSecrets(), ep, tree, mntName); err != nil {
		db.DPrintf(db.MOUNT_ERR, "mountNamed: MountTree err %v", err)
		// If mounting failed, the named is unreachable. Invalidate the cache entry
		// for this realm.
		if err := mc.invalidateNamedMountCacheEntry(realm); err != nil {
			db.DPrintf(db.MOUNT, "Error invalidating named mount cache entry: %v", err)
		}
		return serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v realm failure", realm))
	}
	db.DPrintf(db.MOUNT, "mountNamed [%v]: MountTree ep %v/%v at %v", realm, ep, tree, mntName)
	db.DPrintf(db.WALK_LAT, "mountNamed [%v]: %v MountTree ep %v/%v at %v lat %v", mc.cid, realm, ep, tree, mntName, time.Since(s))
	return nil
}
