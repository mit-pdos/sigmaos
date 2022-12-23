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
		if tip == ip {
			return true
		}
		return false
	default:
		return true
	}
	return true
}
