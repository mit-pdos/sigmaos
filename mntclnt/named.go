package mntclnt

import (
	"fmt"
	gpath "path"

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

// Get named enpoint via netproxy or directly
func (mc *MntClnt) getNamedEndpointRealm(realm sp.Trealm) (*sp.Tendpoint, *serr.Err) {
	// If this mount was passed via the proc env, return it immediately.
	if ep, ok := mc.ndMntCache.Get(mc.pe.GetRealm()); ok {
		db.DPrintf(db.NAMED, "getNamedEndpoint cached %v %v", mc.pe.GetRealm(), ep)
		return ep, nil
	}
	var ep *sp.Tendpoint
	if mc.pe.UseNetProxy {
		ep0, err := mc.npc.GetNamedEndpoint(realm)
		if err != nil {
			if sr, ok := serr.IsErr(err); ok {
				return &sp.Tendpoint{}, sr
			}
			return &sp.Tendpoint{}, serr.NewErrError(err)
		}
		ep = ep0
	} else {
		ep0, err := mc.getRootNamedEndpoint(realm)
		if err != nil {
			return &sp.Tendpoint{}, err
		}
		ep = ep0
	}
	// Cache the newly resolved mount
	mc.ndMntCache.Put(realm, ep)
	db.DPrintf(db.NAMED, "GetNamedEndpointRealm [%v] %v", realm, ep)
	return ep, nil
}

// Get named enpoint directly
func (mc *MntClnt) getRootNamedEndpoint(realm sp.Trealm) (*sp.Tendpoint, *serr.Err) {
	var ep *sp.Tendpoint
	// If this is the root realm, then get the root named.
	if realm == sp.ROOTREALM {
		ep0, err := fsetcd.GetRootNamed(mc.npc.Dial, mc.pe.GetEtcdEndpoints(), realm)
		if err != nil {
			db.DPrintf(db.NAMED_ERR, "getNamedEndpoint [%v] err GetRootNamed %v", realm, ep)
			if sr, ok := serr.IsErr(err); ok {
				return &sp.Tendpoint{}, sr
			}
			return &sp.Tendpoint{}, serr.NewErrError(err)
		}
		ep = ep0
	} else {
		// Otherwise, walk through the root named to find this named's mount.
		if _, rest, err := mc.mnt.resolveMnt(path.Path{"root"}, true); err != nil && len(rest) >= 1 {
			if err := mc.mountNamed(sp.ROOTREALM, "root"); err != nil {
				db.DPrintf(db.NAMED_ERR, "getNamedEndpoint [%v] err mounting root named %v", realm, err)
				return &sp.Tendpoint{}, err
			}
		}
		pn := gpath.Join("root", sp.REALMREL, sp.REALMDREL, sp.REALMSREL, realm.String())
		target, err := mc.pathc.GetFile(pn, mc.pe.GetPrincipal(), sp.OREAD, 0, sp.MAXGETSET, sp.NullFence())
		if err != nil {
			db.DPrintf(db.NAMED_ERR, "getNamedEndpoint [%v] GetFile err %v", realm, err)
			return &sp.Tendpoint{}, serr.NewErrError(err)
		}
		ep, err = sp.NewEndpointFromBytes(target)
		if err != nil {
			return &sp.Tendpoint{}, serr.NewErrError(err)
		}
	}
	return ep, nil
}

func (mc *MntClnt) mountNamed(realm sp.Trealm, name string) *serr.Err {
	ep, err := mc.getNamedEndpointRealm(realm)
	if err != nil {
		db.DPrintf(db.NAMED_ERR, "mountNamed [%v]: getNamedMount err %v", realm, err)
		return err
	}
	if err := mc.AutoMount(mc.pe.GetPrincipal(), ep, path.Path{name}); err != nil {
		db.DPrintf(db.NAMED_ERR, "mountNamed: automount err %v", err)
		// If mounting failed, the named is unreachable. Invalidate the cache entry
		// for this realm.
		mc.ndMntCache.Invalidate(realm)
		return serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v realm failure", realm))
	}
	db.DPrintf(db.NAMED, "mountNamed [%v]: automount ep %v at %v", realm, ep, name)
	return nil
}
