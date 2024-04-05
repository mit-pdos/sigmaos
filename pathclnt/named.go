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

//		// If named has been set, don't bother getting it from fsetcd.
//		if pathc.pe.GetNamedIP() != "" {
//			return sp.Tmount{Addr: []*sp.Taddr{sp.NewTaddr(pathc.pe.GetNamedIP())}}
//		}

func (pathc *PathClnt) GetNamedMount() (*sp.Tmount, error) {
	mnt, err := pathc.getNamedMount(pathc.pe.GetRealm())
	if err != nil {
		db.DPrintf(db.ERROR, "Err getNamedMount [%v]: %v", pathc.pe.GetRealm(), err)
		return mnt, err
	}
	db.DPrintf(db.NAMED, "GetNamedMount %v %v", pathc.pe.GetRealm(), mnt)
	return mnt, nil
}

func (pathc *PathClnt) getNamedMount(realm sp.Trealm) (*sp.Tmount, *serr.Err) {
	// If this mount was passed via the proc config, return it immediately.
	if mnt, ok := pathc.ndMntCache.Get(pathc.pe.GetRealm()); ok {
		db.DPrintf(db.NAMED, "getNamedMount cached %v %v", pathc.pe.GetRealm(), mnt)
		return mnt, nil
	}
	var mnt *sp.Tmount
	var err *serr.Err
	// If this is the root realm, then get the root named.
	if realm == sp.ROOTREALM {
		mnt, err = fsetcd.GetRootNamed(realm, pathc.pe.EtcdIP)
		if err != nil {
			db.DPrintf(db.NAMED_ERR, "getNamedMount [%v] err GetRootNamed %v", realm, mnt)
			return &sp.Tmount{}, err
		}
	} else {
		// Otherwise, walk through the root named to find this named's mount.
		if _, rest, err := pathc.mnt.resolve(path.Path{"root"}, true); err != nil && len(rest) >= 1 {
			if err := pathc.mountNamed(sp.ROOTREALM, "root"); err != nil {
				db.DPrintf(db.NAMED_ERR, "getNamedMount [%v] err mounting root named %v", realm, err)
				return &sp.Tmount{}, err
			}
		}
		pn := gpath.Join("root", sp.REALMREL, sp.REALMDREL, sp.REALMSREL, realm.String())
		target, err := pathc.GetFile(pn, pathc.pe.GetPrincipal(), sp.OREAD, 0, sp.MAXGETSET, sp.NullFence())
		if err != nil {
			db.DPrintf(db.NAMED_ERR, "getNamedMount [%v] GetFile err %v", realm, err)
			return &sp.Tmount{}, serr.NewErrError(err)
		}
		var sr *serr.Err
		mnt, sr = sp.NewMountFromBytes(target)
		if sr != nil {
			return &sp.Tmount{}, sr
		}
	}
	// Cache the newly resolved mount
	pathc.ndMntCache.Put(realm, mnt)
	db.DPrintf(db.NAMED, "getNamedMount [%v] %v", realm, mnt)
	return mnt, nil
}

func (pathc *PathClnt) mountNamed(realm sp.Trealm, name string) *serr.Err {
	mnt, err := pathc.getNamedMount(realm)
	if err != nil {
		db.DPrintf(db.NAMED_ERR, "mountNamed [%v]: getNamedMount err %v", realm, err)
		return err
	}
	if err := pathc.autoMount(pathc.pe.GetPrincipal(), mnt, path.Path{name}); err != nil {
		db.DPrintf(db.NAMED_ERR, "mountNamed: automount err %v", err)
		// If mounting failed, the named is unreachable. Invalidate the cache entry
		// for this realm.
		pathc.ndMntCache.Invalidate(realm)
		return serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v realm failure", realm))
	}
	db.DPrintf(db.NAMED, "mountNamed [%v]: automount mnt %v at %v", realm, mnt, name)
	return nil
}
