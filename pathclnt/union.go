package pathclnt

import (
	db "sigmaos/debug"
	"sigmaos/sessp"
	"sigmaos/reader"
	sp "sigmaos/sigmap"
	"sigmaos/union"
)

func (pathc *PathClnt) unionScan(fid sp.Tfid, name, q string) (sp.Tfid, *sessp.Err) {
	fid1, _, err := pathc.FidClnt.Walk(fid, []string{name})
	if err != nil {
		return sp.NoFid, err
	}
	defer pathc.FidClnt.Clunk(fid1)

	target, err := pathc.readlink(fid1)
	if err != nil {
		return sp.NoFid, err
	}
	mnt, err := sp.MkMount(target)
	if err != nil {
		return sp.NoFid, nil
	}
	db.DPrintf(db.WALK, "unionScan: mnt: %v\n", mnt)
	if union.UnionMatch(q, mnt) {
		fid2, _, err := pathc.FidClnt.Walk(fid, []string{name})
		if err != nil {
			return sp.NoFid, err
		}
		return fid2, nil
	}
	return sp.NoFid, nil
}

// Caller is responsible for clunking fid
func (pathc *PathClnt) unionLookup(fid sp.Tfid, q string) (sp.Tfid, *sessp.Err) {
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
			db.DPrintf(db.WALK, "unionScan %v err %v\n", st.Name, err)
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
	return rfid, sessp.MkErr(sessp.TErrNotfound, q)
}
