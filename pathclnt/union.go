package pathclnt

import (
	"ulambda/fidclnt"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/reader"
)

func (pathc *PathClnt) walkUnion(fid np.Tfid, path []string) (np.Tfid, []string, *np.Err) {
	fid2, err := pathc.unionLookup(fid, path[0])
	if err != nil {
		return np.NoFid, path, err
	}
	return fid2, path[1:], nil
}

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

func (pathc *PathClnt) unionScan(fid np.Tfid, de *np.Stat, q string) (np.Tfid, *np.Err) {
	fid2, _, err := pathc.FidClnt.Walk(fid, []string{de.Name})
	if err != nil {
		return np.NoFid, err
	}
	defer pathc.FidClnt.Clunk(fid2)
	target, err := pathc.readlink(fid2)
	if err != nil {
		return np.NoFid, err
	}
	if pathc.unionMatch(q, target) {
		fid3, _, err := pathc.FidClnt.Walk(fid2, []string{de.Name})
		if err != nil {
			return np.NoFid, err
		}
		return fid3, nil
	}
	return np.NoFid, nil
}

func (pathc *PathClnt) unionLookup(fid np.Tfid, q string) (np.Tfid, *np.Err) {
	_, err := pathc.FidClnt.Open(fid, np.OREAD)
	if err != nil {
		return np.NoFid, err
	}
	rdr := reader.MakeReader(pathc.FidClnt, fid, pathc.chunkSz)
	defer rdr.Close()
	for {
		de := &np.Stat{}
		err := npcodec.UnmarshalReader(rdr, de)
		if err != nil && np.IsErrEOF(err) {
			break
		}
		if err != nil {
			return np.NoFid, err
		}
		fid2, err := pathc.unionScan(fid, de, q)
		if err != nil {
			return np.NoFid, err
		}
		if fid2 != np.NoFid { // success
			return fid2, nil
		}
	}
	return np.NoFid, nil
}
