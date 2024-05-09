package pathclnt

import (
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func (pathc *PathClnt) walkSymlink1(fid sp.Tfid, resolved, left path.Path) (path.Path, *serr.Err) {
	// XXX change how we readlink; getfile?
	target, err := pathc.readlink(fid)
	if err != nil {
		db.DPrintf(db.WALK, "walksymlink1 %v err %v\n", fid, err)
		return left, err
	}
	var p path.Path
	ep, error := sp.NewEndpointFromBytes(target)
	if error == nil {
		db.DPrintf(db.WALK, "walksymlink1 %v ep %v err %v\n", fid, ep, err)
		err := pathc.mntclnt.AutoMount(pathc.FidClnt.Lookup(fid).Principal(), ep, resolved)
		if err != nil {
			db.DPrintf(db.WALK, "automount %v %v err %v\n", resolved, ep, err)
			return left, err
		}
		p = append(resolved, left...)
	} else {
		db.DPrintf(db.WALK, "walksymlink1 %v NewMount err %v\n", fid, err)
		p = append(path.Split(string(target)), left...)
	}
	return p, nil
}
