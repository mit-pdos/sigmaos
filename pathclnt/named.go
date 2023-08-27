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

func (pathc *PathClnt) GetMntNamed(uname sp.Tuname) sp.Tmount {
	if pathc.scfg.Realm == sp.ROOTREALM {
		mnt, err := fsetcd.GetRootNamed(pathc.scfg)
		if err != nil {
			db.DFatalf("GetMntNamed() GetRootNamed %v err %v\n", pathc.scfg.Realm, err)
		}
		db.DPrintf(db.NAMED, "GetMntNamed %v %v\n", pathc.scfg.Realm, mnt)
		return mnt
	} else {
		mnt, err := pathc.getRealmNamed(uname)
		if err != nil {
			db.DFatalf("GetMntNamed() getRealmNamed %v err %v\n", pathc.scfg.Realm, err)
		}
		db.DPrintf(db.NAMED, "GetMntNamed %v %v\n", pathc.scfg.Realm, mnt)
		return mnt
	}
}

func (pathc *PathClnt) mountNamed(p path.Path, uname sp.Tuname) *serr.Err {
	db.DPrintf(db.NAMED, "mountNamed %v: %v\n", pathc.scfg.Realm, p)
	if pathc.scfg.Realm == sp.ROOTREALM {
		return pathc.mountRootNamed(sp.NAME, uname)
	} else {
		return pathc.mountRealmNamed(uname)
	}
	return nil
}

func (pathc *PathClnt) mountRootNamed(name string, uname sp.Tuname) *serr.Err {
	db.DPrintf(db.NAMED, "mountRootNamed %v\n", name)
	mnt, err := fsetcd.GetRootNamed(pathc.scfg)
	if err == nil {
		pn := path.Path{name}
		if err := pathc.autoMount(uname, mnt, pn); err == nil {
			db.DPrintf(db.NAMED, "mountRootNamed: automount %v at %v\n", mnt, pn)
			return nil
		} else {
			db.DPrintf(db.NAMED, "mountRootNamed: automount err %v\n", err)
			return err
		}
		return nil
	}
	db.DPrintf(db.NAMED, "mountRootNamed: GetNamed err %v\n", err)
	return err
}

func (pathc *PathClnt) getRealmNamed(uname sp.Tuname) (sp.Tmount, *serr.Err) {
	if _, rest, err := pathc.mnt.resolve(path.Path{"root"}, true); err != nil && len(rest) >= 1 {
		if err := pathc.mountRootNamed("root", uname); err != nil {
			db.DPrintf(db.NAMED, "getRealmNamed %v err mounting root named %v\n", pathc.scfg.Realm, err)
			return sp.Tmount{}, err
		}
	}
	pn := gpath.Join("root", sp.REALMDREL, sp.REALMSREL, pathc.scfg.Realm.String())
	target, err := pathc.GetFile(pn, uname, sp.OREAD, 0, sp.MAXGETSET)
	if err != nil {
		db.DPrintf(db.NAMED, "getRealmNamed %v err %v\n", pathc.scfg.Realm, err)
		return sp.Tmount{}, serr.MkErrError(err)
	}
	mnt, sr := sp.MkMount(target)
	if sr != nil {
		return sp.Tmount{}, sr
	}
	db.DPrintf(db.NAMED, "getRealmNamed %v %v\n", pathc.scfg.Realm, mnt)
	return mnt, nil
}

func (pathc *PathClnt) mountRealmNamed(uname sp.Tuname) *serr.Err {
	mnt, err := pathc.getRealmNamed(uname)
	if err != nil {
		db.DPrintf(db.NAMED, "mountRealmNamed: getRrealmNamed err %v\n", err)
		return err
	}
	if err := pathc.autoMount(uname, mnt, path.Path{sp.NAME}); err == nil {
		db.DPrintf(db.NAMED, "mountRealmNamed: automount mnt %v at %v\n", mnt, sp.NAME)
		return nil
	}
	db.DPrintf(db.NAMED, "mountRealmNamed: automount err %v\n", err)
	return serr.MkErr(serr.TErrRetry, fmt.Sprintf("%v realm failure", pathc.scfg.Realm))
}
