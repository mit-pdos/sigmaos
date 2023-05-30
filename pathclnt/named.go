package pathclnt

import (
	// "time"

	db "sigmaos/debug"
	"sigmaos/etcdclnt"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func (pathc *PathClnt) mountNamed(p path.Path) *serr.Err {
	_, rest, err := pathc.mnt.resolve(p, false)
	if err != nil && len(rest) >= 1 && rest[0] == sp.NAMEDV1 {
		db.DPrintf(db.NAMEDV1, "mountNamed: %v\n", p)
		mnt, err := etcdclnt.GetNamed()
		if err != nil {
			db.DPrintf(db.NAMEDV1, "mountNamed: GetNamed err %v\n", err)
			return err
		}
		db.DPrintf(db.NAMEDV1, "mountNamed mnt %v err %v\n", mnt, err)
		if err := pathc.autoMount("", mnt, path.Path{sp.NAMEDV1}); err != nil {
			db.DPrintf(db.NAMEDV1, "automount err %v\n", err)
			return err
		}
		db.DPrintf(db.NAMEDV1, "mount paths %v\n", pathc.mnt.mountedPaths())
	}
	return nil
}
