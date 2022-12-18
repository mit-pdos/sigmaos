package pathclnt

import (
	"errors"
	"io"

	db "sigmaos/debug"
	"sigmaos/fcall"
	"sigmaos/fidclnt"
	"sigmaos/reader"
	sp "sigmaos/sigmap"
	"sigmaos/spcodec"
)

func (pathc *PathClnt) UnionMatch(q, name string) bool {
	switch q {
	case "~any":
		return true
	case "~ip":
		ip, err := fidclnt.LocalIP()
		if err != nil {
			return false
		}
		tip := sp.TargetIp(name)
		if tip == "" {
			tip = ip
		}
		if ok := sp.IsRemoteTarget(name); ok && tip == ip {
			return true
		}
		return false
	default:
		return true
	}
	return true
}

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
	if pathc.UnionMatch(q, target) {
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
	for {
		de, err := spcodec.UnmarshalDirEnt(drdr)
		if err != nil && errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return sp.NoFid, err
		}
		fid1, err := pathc.unionScan(fid, de.Name, q)
		if err != nil {
			db.DPrintf("unionScan %v err %v\n", de.Name, err)
			continue
		}
		if fid1 != sp.NoFid { // success
			return fid1, nil
		}
	}
	return sp.NoFid, fcall.MkErr(fcall.TErrNotfound, q)
}
