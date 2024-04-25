package pathclnt

import (
	"fmt"
	gpath "path"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	path "sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func (pathc *PathClnt) GetNamedEndpoint() (*sp.Tendpoint, error) {
	ep, err := pathc.getNamedEndpoint(pathc.pe.GetRealm())
	if err != nil {
		db.DPrintf(db.ERROR, "Err getNamedEndpoint [%v]: %v", pathc.pe.GetRealm(), err)
		return ep, err
	}
	db.DPrintf(db.NAMED, "GetNamedEndpoint %v %v", pathc.pe.GetRealm(), ep)
	return ep, nil
}

func (pathc *PathClnt) getNamedEndpoint(realm sp.Trealm) (*sp.Tendpoint, *serr.Err) {
	// If this mount was passed via the proc config, return it immediately.
	if ep, ok := pathc.ndMntCache.Get(pathc.pe.GetRealm()); ok {
		db.DPrintf(db.NAMED, "getNamedEndpoint cached %v %v", pathc.pe.GetRealm(), ep)
		return ep, nil
	}
	var ep *sp.Tendpoint
	var err *serr.Err
	// If this is the root realm, then get the root named.
	if realm == sp.ROOTREALM {
		ep, err = fsetcd.GetRootNamed(pathc.GetNetProxyClnt(), pathc.pe.GetEtcdEndpoints(), realm)
		if err != nil {
			db.DPrintf(db.NAMED_ERR, "getNamedEndpoint [%v] err GetRootNamed %v", realm, ep)
			return &sp.Tendpoint{}, err
		}
	} else {
		// Otherwise, walk through the root named to find this named's mount.
		if _, rest, err := pathc.mnt.resolve(path.Path{"root"}, true); err != nil && len(rest) >= 1 {
			if err := pathc.mountNamed(sp.ROOTREALM, "root"); err != nil {
				db.DPrintf(db.NAMED_ERR, "getNamedEndpoint [%v] err mounting root named %v", realm, err)
				return &sp.Tendpoint{}, err
			}
		}
		pn := gpath.Join("root", sp.REALMREL, sp.REALMDREL, sp.REALMSREL, realm.String())
		target, err := pathc.GetFile(pn, pathc.pe.GetPrincipal(), sp.OREAD, 0, sp.MAXGETSET, sp.NullFence())
		if err != nil {
			db.DPrintf(db.NAMED_ERR, "getNamedEndpoint [%v] GetFile err %v", realm, err)
			return &sp.Tendpoint{}, serr.NewErrError(err)
		}
		var sr *serr.Err
		ep, sr = sp.NewEndpointFromBytes(target)
		if sr != nil {
			return &sp.Tendpoint{}, sr
		}
	}
	// Cache the newly resolved mount
	pathc.ndMntCache.Put(realm, ep)
	db.DPrintf(db.NAMED, "getNamedEndpoint [%v] %v", realm, ep)
	return ep, nil
}

func (pathc *PathClnt) mountNamed(realm sp.Trealm, name string) *serr.Err {
	ep, err := pathc.getNamedEndpoint(realm)
	if err != nil {
		db.DPrintf(db.NAMED_ERR, "mountNamed [%v]: getNamedMount err %v", realm, err)
		return err
	}
	if err := pathc.autoMount(pathc.pe.GetPrincipal(), ep, path.Path{name}); err != nil {
		db.DPrintf(db.NAMED_ERR, "mountNamed: automount err %v", err)
		// If mounting failed, the named is unreachable. Invalidate the cache entry
		// for this realm.
		pathc.ndMntCache.Invalidate(realm)
		return serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v realm failure", realm))
	}
	db.DPrintf(db.NAMED, "mountNamed [%v]: automount ep %v at %v", realm, ep, name)
	return nil
}
