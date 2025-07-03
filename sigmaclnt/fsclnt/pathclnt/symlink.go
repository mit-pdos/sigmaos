package pathclnt

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func (pathc *PathClnt) walkReadSymlink(fid sp.Tfid, resolved, left path.Tpathname) (path.Tpathname, *serr.Err) {
	s := time.Now()
	target, err := pathc.FidClnt.GetFile(fid, path.Tpathname{}, sp.OREAD, 0, sp.MAXGETSET, false, sp.NullFence())
	if err != nil {
		db.DPrintf(db.WALK, "walkReadSymlink %v err %v\n", fid, err)
		return left, err

	}
	var p path.Tpathname
	ep, error := sp.NewEndpointFromBytes(target)
	if error == nil {

		db.DPrintf(db.WALK_LAT, "walkReadSymlink %v %v %v ep %v lat %v\n", pathc.cid, fid, resolved, ep, time.Since(s))

		error := pathc.mntclnt.MountTree(pathc.pe.GetSecrets(), ep, ep.Root, resolved.String())
		if error != nil {
			db.DPrintf(db.WALK_ERR, "automount %v %v err %v\n", resolved, ep, error)
			return left, error.(*serr.Err)

		}
		p = append(resolved, left...)
	} else {
		db.DPrintf(db.WALK, "walkReadSymlink %v NewMount err %v\n", fid, err)
		p = append(path.Split(string(target)), left...)
	}
	return p, nil
}

func (pathc *PathClnt) walkReadSymlink1(fid sp.Tfid, resolved path.Tpathname) (sp.Tfid, path.Tpathname, *serr.Err) {
	s := time.Now()
	target, sr := pathc.FidClnt.GetFile(fid, path.Tpathname{}, sp.OREAD, 0, sp.MAXGETSET, false, sp.NullFence())
	if sr != nil {
		db.DPrintf(db.WALK, "walkReadSymlink1: GetFile %v err %v\n", fid, sr)
		return sp.NoFid, nil, sr
	}
	ep, err := sp.NewEndpointFromBytes(target)
	if err == nil { // an endpoint file
		db.DPrintf(db.WALK_LAT, "walkReadSymlink1: %v %v %v ep %v lat %v\n", pathc.cid, fid, resolved, ep, time.Since(s))
		fid, err := pathc.mntclnt.MountTreeFid(pathc.pe.GetSecrets(), ep, ep.Root, resolved.String())
		if err != nil {
			db.DPrintf(db.WALK_ERR, "walkReadSymlink1: automount %v %v err %v\n", resolved, ep, err)
			return sp.NoFid, nil, err.(*serr.Err)
		}
		fid1, sr1 := pathc.FidClnt.Clone(fid)
		if sr1 != nil {
			db.DPrintf(db.WALK_ERR, "walkReadSymlink1: clone %v %v err %v\n", fid, fid1, sr1)
			return sp.NoFid, nil, sr1
		}
		return fid1, nil, nil
	} else { // a true symlink
		db.DPrintf(db.WALK, "walkReadSymlink1: %v NewMount err %v\n", fid, err)
		pn := path.Split(string(target))
		return sp.NoFid, pn, nil
	}
}
