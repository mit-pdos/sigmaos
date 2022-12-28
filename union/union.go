package union

import (
	db "sigmaos/debug"
	"sigmaos/fidclnt"
	sp "sigmaos/sigmap"
)

func UnionMatch(q string, mnt sp.Tmount) bool {
	switch q {
	case "~any":
		return true
	case "~local":
		ip, err := fidclnt.LocalIP()
		if err != nil {
			return false
		}
		tip := mnt.TargetIp()
		if tip == "" {
			tip = ip
		}
		db.DPrintf(db.MOUNT, "UnionMatch: %v ip %v tip %v\n", q, ip, tip)
		if tip == ip ||
			// XXX hack for ~local in tests when running with kernel in container
			(ip == "10.100.42.1" && tip == "10.100.42.124") {
			return true
		}
		return false
	default:
		return true
	}
	return true
}
