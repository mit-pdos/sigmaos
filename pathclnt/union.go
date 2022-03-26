package pathclnt

import (
	"errors"
	"io"

	db "ulambda/debug"
	"ulambda/fidclnt"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/reader"
)

func (pathc *PathClnt) unionMatch(q, name string) bool {
	switch q {
	case "~any":
		return true
	case "~ip":
		ip, err := fidclnt.LocalIP()
		if err != nil {
			return false
		}
		// XXX need to match on ip, but for now at least
		// check that name is a remote target
		if IsRemoteTarget(name) && ip == ip {
			return true
		}
		return false
	default:
		return true
	}
	return true
}

func (pathc *PathClnt) unionScan(fid np.Tfid, name, q string) (np.Tfid, *np.Err) {
	fid1, _, err := pathc.FidClnt.Walk(fid, []string{name})
	if err != nil {
		return np.NoFid, err
	}
	defer pathc.FidClnt.Clunk(fid1)
	target, err := pathc.readlink(fid1)
	if err != nil {
		return np.NoFid, err
	}
	db.DLPrintf("WALK", "unionScan: target: %v\n", target)
	if pathc.unionMatch(q, target) {
		fid2, _, err := pathc.FidClnt.Walk(fid1, []string{name})
		if err != nil {
			return np.NoFid, err
		}
		return fid2, nil
	}
	return np.NoFid, nil
}

// Caller is responsible for clunking fid
func (pathc *PathClnt) unionLookup(fid np.Tfid, q string) (np.Tfid, *np.Err) {
	_, err := pathc.FidClnt.Open(fid, np.OREAD)
	if err != nil {
		return np.NoFid, err
	}
	rdr := reader.MakeReader(pathc.FidClnt, "", fid, pathc.chunkSz)
	for {
		de, err := npcodec.UnmarshalDirEnt(rdr)
		if err != nil && errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return np.NoFid, err
		}
		fid1, err := pathc.unionScan(fid, de.Name, q)
		if err != nil {
			db.DLPrintf("unionScan %v err %v\n", de.Name, err)
			continue
		}
		if fid1 != np.NoFid { // success
			return fid1, nil
		}
	}
	return np.NoFid, np.MkErr(np.TErrNotfound, q)
}
