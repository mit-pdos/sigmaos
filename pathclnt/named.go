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
//		if pathc.pcfg.GetNamedIP() != "" {
//			return sp.Tmount{Addr: []*sp.Taddr{sp.NewTaddr(pathc.pcfg.GetNamedIP())}}
//		}

func (pathc *PathClnt) GetNamedMount() sp.Tmount {
	// If this mount was passed via the proc config, return it immediately.
	if mnt, ok := pathc.pcfg.GetNamedMount(); ok {
		db.DPrintf(db.NAMED, "GetNamedMount cached %v %v", pathc.pcfg.GetRealm(), mnt)
		return mnt
	}
	mnt, err := pathc.getNamedMount(pathc.pcfg.GetRealm())
	if err != nil {
		db.DFatalf("Err getNamedMount [%v]: %v", pathc.pcfg.GetRealm(), err)
	}
	// TODO: cache in pcfg?
	db.DPrintf(db.NAMED, "GetNamedMount %v %v", pathc.pcfg.GetRealm(), mnt)
	return mnt
}

func (pathc *PathClnt) getNamedMount(realm sp.Trealm) (sp.Tmount, *serr.Err) {
	// If this is the root realm, then get the root named.
	if realm == sp.ROOTREALM {
		mnt, err := fsetcd.GetRootNamed(realm, pathc.pcfg.EtcdIP)
		if err != nil {
			db.DPrintf(db.NAMED_ERR, "getNamedMount [%v] err GetRootNamed %v", realm, mnt)
		}
		return mnt, err
	}
	// Otherwise, walk through the root named to find this named's mount.
	if _, rest, err := pathc.mnt.resolve(path.Path{"root"}, true); err != nil && len(rest) >= 1 {
		if err := pathc.mountNamed(sp.ROOTREALM, "root"); err != nil {
			db.DPrintf(db.NAMED_ERR, "getNamedMount [%v] err mounting root named %v", realm, err)
			return sp.Tmount{}, err
		}
	}
	pn := gpath.Join("root", sp.REALMDREL, sp.REALMSREL, realm.String())
	target, err := pathc.GetFile(pn, pathc.pcfg.GetUname(), sp.OREAD, 0, sp.MAXGETSET)
	if err != nil {
		db.DPrintf(db.NAMED_ERR, "getNamedMount [%v] GetFile err %v", realm, err)
		return sp.Tmount{}, serr.NewErrError(err)
	}
	mnt, sr := sp.NewMount(target)
	if sr != nil {
		return sp.Tmount{}, sr
	}
	db.DPrintf(db.NAMED, "getNamedMount [%v] %v", realm, mnt)
	return mnt, nil
}

func (pathc *PathClnt) mountNamed(realm sp.Trealm, name string) *serr.Err {
	mnt, err := pathc.getNamedMount(realm)
	if err != nil {
		db.DPrintf(db.NAMED_ERR, "mountNamed [%v]: getNamedMount err %v", realm, err)
		return err
	}
	if err := pathc.autoMount(pathc.pcfg.GetUname(), mnt, path.Path{name}); err != nil {
		db.DPrintf(db.NAMED_ERR, "mountRealmNamed: automount err %v\n", err)
		return serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v realm failure", realm))
	}
	db.DPrintf(db.NAMED, "mountNamed [%v]: automount mnt %v at %v\n", realm, mnt, name)
	return nil
}
