package pathclnt

import (
	db "sigmaos/debug"
	"sigmaos/reader"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func (pathc *PathClnt) IsLocalMount(mnt *sp.Tmount) (bool, error) {
	outerIP := pathc.pe.GetOuterContainerIP()
	tip, _ := mnt.TargetIPPort(0)
	if tip == "" {
		tip = outerIP
	}
	db.DPrintf(db.MOUNT, "IsLocalMount: tip %v outerIP %v\n", tip, outerIP)
	if tip == outerIP {
		return true, nil
	}
	return false, nil
}

func (pathc *PathClnt) unionScan(fid sp.Tfid, name, q string) (sp.Tfid, *serr.Err) {
	fid1, _, err := pathc.FidClnt.Walk(fid, []string{name})
	if err != nil {
		db.DPrintf(db.WALK, "unionScan: error walk: %v", err)
		return sp.NoFid, err
	}
	defer pathc.FidClnt.Clunk(fid1)

	target, err := pathc.readlink(fid1)
	if err != nil {
		db.DPrintf(db.WALK, "unionScan: Err readlink %v\n", err)
		return sp.NoFid, err
	}
	db.DPrintf(db.WALK, "unionScan: %v target: %v\n", name, string(target))
	mnt, err := sp.NewMountFromBytes(target)
	if err != nil {
		db.DPrintf(db.WALK, "unionScan NewMount err %v", err)
		return sp.NoFid, nil
	}
	db.DPrintf(db.WALK, "unionScan: %v mnt: %v\n", name, mnt)
	ok, _ := pathc.IsLocalMount(mnt)
	if q == "~any" || ok {
		fid2, _, err := pathc.FidClnt.Walk(fid, []string{name})
		if err != nil {
			db.DPrintf(db.WALK, "unionScan UnionMatch Walk %v err %v", fid, err)
			return sp.NoFid, err
		}
		return fid2, nil
	}
	db.DPrintf(db.WALK, "unionScan NoFID")
	return sp.NoFid, nil
}

// Caller is responsible for clunking fid
func (pathc *PathClnt) unionLookup(fid sp.Tfid, q string) (sp.Tfid, *serr.Err) {
	_, err := pathc.FidClnt.Open(fid, sp.OREAD)
	if err != nil {
		db.DPrintf(db.WALK, "unionLookup open %v fid %v err %v", q, fid, err)
		return sp.NoFid, err
	}
	rdr := reader.NewReader(newRdr(pathc.FidClnt, fid, sp.NullFence()), "")
	drdr := reader.MkDirReader(rdr)
	rfid := sp.NoFid
	db.DPrintf(db.WALK, "unionLookup ReadDir %v search %v fid %v", fid, q, fid)
	_, error := reader.ReadDir(drdr, func(st *sp.Stat) (bool, error) {
		db.DPrintf(db.WALK, "unionScan %v check %v", q, st.Name)
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
	db.DPrintf(db.WALK, "unionLookup error ReadDir fid %v rfid %v err %v", fid, rfid, error)
	return rfid, serr.NewErr(serr.TErrNotfound, q)
}
