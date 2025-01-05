package pathclnt

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func (pathc *PathClnt) walkSymlink1(fid sp.Tfid, resolved, left path.Tpathname) (path.Tpathname, *serr.Err) {
	s := time.Now()
	target, err := pathc.FidClnt.GetFile(fid, path.Tpathname{}, sp.OREAD, 0, sp.MAXGETSET, false, sp.NullFence())
	if err != nil {
		db.DPrintf(db.WALK, "walksymlink1 %v err %v\n", fid, err)
		return left, err
	}
	var p path.Tpathname
	ep, error := sp.NewEndpointFromBytes(target)
	if error == nil {
		db.DPrintf(db.WALK_LAT, "walksymlink1 %v %v %v ep %v lat %v\n", pathc.cid, fid, resolved, ep, time.Since(s))

		error := pathc.mntclnt.MountTree(pathc.pe.GetSecrets(), ep, ep.Root, resolved.String())
		if error != nil {
			db.DPrintf(db.WALK, "automount %v %v err %v\n", resolved, ep, error)
			return left, error.(*serr.Err)
		}
		p = append(resolved, left...)
	} else {
		db.DPrintf(db.WALK, "walksymlink1 %v NewMount err %v\n", fid, err)
		p = append(path.Split(string(target)), left...)
	}
	return p, nil
}
