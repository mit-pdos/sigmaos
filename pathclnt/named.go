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

func (pathc *PathClnt) GetMntNamed() sp.Tmount {
	if pathc.realm == sp.ROOTREALM {
		mnt, err := fsetcd.GetRootNamed()
		if err != nil {
			db.DFatalf("GetMntNamed() %v err %v\n", pathc.realm, err)
		}
		db.DPrintf(db.NAMED, "GetMntNamed %v %v\n", pathc.realm, mnt)
		return mnt
	} else {
		mnt, err := pathc.getRealmNamed()
		if err != nil {
			db.DFatalf("GetMntNamed() %v err %v\n", pathc.realm, err)
		}
		db.DPrintf(db.NAMED, "GetMntNamed %v %v\n", pathc.realm, mnt)
		return mnt
	}
}

func (pathc *PathClnt) mountNamed(p path.Path) *serr.Err {
	db.DPrintf(db.NAMED, "mountNamed %v: %v\n", pathc.realm, p)
	if pathc.realm == sp.ROOTREALM {
		return pathc.mountRootNamed(sp.NAME)
	} else {
		return pathc.mountRealmNamed()
	}
	return nil
}

func (pathc *PathClnt) mountRootNamed(name string) *serr.Err {
	db.DPrintf(db.NAMED, "mountRootNamed %v\n", name)
	mnt, err := fsetcd.GetRootNamed()
	if err == nil {
		pn := path.Path{name}
		if err := pathc.autoMount("", mnt, pn); err == nil {
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

func (pathc *PathClnt) getRealmNamed() (sp.Tmount, *serr.Err) {
	if _, rest, err := pathc.mnt.resolve(path.Path{"root"}, true); err != nil && len(rest) >= 1 {
		if err := pathc.mountRootNamed("root"); err != nil {
			return sp.Tmount{}, err
		}
	}
	pn := gpath.Join("root", sp.REALMDREL, sp.REALMSREL, pathc.realm.String())
	target, err := pathc.GetFile(pn, sp.OREAD, 0, sp.MAXGETSET)
	if err != nil {
		db.DPrintf(db.NAMED, "getRealmNamed %v err %v\n", pathc.realm, err)
		return sp.Tmount{}, serr.MkErrError(err)
	}
	mnt, sr := sp.MkMount(target)
	if sr != nil {
		return sp.Tmount{}, sr
	}
	db.DPrintf(db.NAMED, "getRealmNamed %v %v\n", pathc.realm, mnt)
	return mnt, nil
}

func (pathc *PathClnt) mountRealmNamed() *serr.Err {
	mnt, err := pathc.getRealmNamed()
	if err != nil {
		db.DPrintf(db.NAMED, "mountRealmNamed: getRrealmNamed err %v\n", err)
		return err
	}
	if err := pathc.autoMount("", mnt, path.Path{sp.NAME}); err == nil {
		db.DPrintf(db.NAMED, "mountRealmNamed: automount mnt %v at %v\n", mnt, sp.NAME)
		return nil
	}
	db.DPrintf(db.NAMED, "mountRealmNamed: automount err %v\n", err)
	return serr.MkErr(serr.TErrRetry, fmt.Sprintf("%v realm failure", pathc.realm))
}
