package pathclnt

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/util/spstats"
)

func (pathc *PathClnt) walkEndpoint(ep *sp.Tendpoint, resolved path.Tpathname) (sp.Tfid, *serr.Err) {
	fid, err := pathc.mntclnt.MountTreeFid(pathc.pe.GetSecrets(), ep, ep.Root, resolved.String())
	if err != nil {
		db.DPrintf(db.WALK_ERR, "walkEndpoint: automount %v %v err %v\n", resolved, ep, err)
		return sp.NoFid, err.(*serr.Err)
	}
	fid1, sr1 := pathc.FidClnt.Clone(fid)
	if sr1 != nil {
		db.DPrintf(db.WALK_ERR, "walkEndpoint: clone %v %v err %v\n", fid, fid1, sr1)
		return sp.NoFid, sr1
	}
	return fid1, nil
}

func (pathc *PathClnt) walkReadSymfile(fid sp.Tfid, resolved path.Tpathname) (sp.Tfid, path.Tpathname, *serr.Err) {
	s := time.Now()
	spstats.Inc(&pathc.pcstats.NreadSym, 1)
	target, sr := pathc.FidClnt.GetFile(fid, path.Tpathname{}, sp.OREAD, 0, sp.MAXGETSET, false, sp.NullFence())
	if sr != nil {
		db.DPrintf(db.WALK, "walkReadSymfile: GetFile %v err %v", fid, sr)
		return sp.NoFid, nil, sr
	}
	ep, err := sp.NewEndpointFromBytes(target)
	if err == nil { // an endpoint file
		spstats.Inc(&pathc.pcstats.NwalkEP, 1)
		fid, err := pathc.walkEndpoint(ep, resolved)
		db.DPrintf(db.WALK_LAT, "walkReadSymfile: ep %v %v %v ep %v lat %v", pathc.cid, fid, resolved, ep, time.Since(s))
		return fid, nil, err
	} else { // a true symlink
		spstats.Inc(&pathc.pcstats.NwalkSym, 1)
		pn := path.Split(string(target))
		db.DPrintf(db.WALK, "walkReadSymfile: sym target '%v'", pn)
		return sp.NoFid, pn, nil
	}
}
