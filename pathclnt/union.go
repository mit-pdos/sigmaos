package pathclnt

import (
	db "sigmaos/debug"
	"sigmaos/fcall"
	"sigmaos/reader"
	sp "sigmaos/sigmap"
	"sigmaos/union"
)

func (pathc *PathClnt) unionScan(fid sp.Tfid, name, q string) (sp.Tfid, *fcall.Err) {
	fid1, _, err := pathc.FidClnt.Walk(fid, []string{name})
	if err != nil {
		return sp.NoFid, err
	}
	defer pathc.FidClnt.Clunk(fid1)
	target, err := pathc.readlink(fid1)
	if err != nil {
		return sp.NoFid, err
	}
	db.DPrintf("WALK", "unionScan: target: %v\n", target)
	if union.UnionMatch(q, target) {
		fid2, _, err := pathc.FidClnt.Walk(fid, []string{name})
		if err != nil {
			return sp.NoFid, err
		}
		return fid2, nil
	}
	return sp.NoFid, nil
}

// Caller is responsible for clunking fid
func (pathc *PathClnt) unionLookup(fid sp.Tfid, q string) (sp.Tfid, *fcall.Err) {
	_, err := pathc.FidClnt.Open(fid, sp.OREAD)
	if err != nil {
		return sp.NoFid, err
	}
	rdr := reader.MakeReader(pathc.FidClnt, "", fid, pathc.chunkSz)
	drdr := rdr.NewDirReader()
	rfid := sp.NoFid
	_, error := reader.ReadDir(drdr, func(st *sp.Stat) (bool, error) {
		fid1, err := pathc.unionScan(fid, st.Name, q)
		if err != nil {
			db.DPrintf("WALK", "unionScan %v err %v\n", st.Name, err)
			// ignore error; keep going
			return false, nil
		}
		if fid1 != sp.NoFid { // success
			rfid = fid1
			return true, nil
		}
		return false, nil
	})
	if error == nil && rfid != sp.NoFid {
		return rfid, nil
	}
	return rfid, fcall.MkErr(fcall.TErrNotfound, q)
}
