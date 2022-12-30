package union

import (
	"strings"

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
			// XXX what if multiple containers locally?
			(ip == "10.100.42.1" && strings.HasPrefix(tip, "10.100.42.")) {
			return true
		}
		return false
	default:
		return true
	}
	return true
}
