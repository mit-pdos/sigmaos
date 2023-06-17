package pathclnt

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

const MAXRETRY = 10

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

func (pathc *PathClnt) resolveNamed(p path.Path) *serr.Err {
	_, rest, err := pathc.mnt.resolve(p, true)
	// db.DPrintf(db.NAMED, "%p: resolveNamed: %v r %v err %v\n", pathc, p, rest, err)
	if err != nil && len(rest) >= 1 && rest[0] == sp.NAME {
		pathc.mountNamed(p)
	}
	return nil
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
	for i := 0; i < MAXRETRY; i++ {
		db.DPrintf(db.NAMED, "mountRootNamed %d: %v\n", i, name)
		mnt, err := fsetcd.GetRootNamed()
		if err == nil {
			pn := path.Path{name}
			if err := pathc.autoMount("", mnt, pn); err == nil {
				db.DPrintf(db.NAMED, "mountRootNamed: automount %v at %v\n", mnt, pn)
				return nil
			} else {
				db.DPrintf(db.NAMED, "mountRootNamed: automount err %v\n", err)
			}
		} else {
			db.DPrintf(db.NAMED, "mountRootNamed: GetNamed err %v\n", err)
		}
		time.Sleep(1 * time.Second)
	}
	return serr.MkErr(serr.TErrRetry, fmt.Sprintf("%v failure", sp.ROOTREALM))
}

// XXX retry mounting realm's named on error
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
