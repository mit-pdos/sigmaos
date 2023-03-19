package union

import (
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

func UnionMatch(lip string, q string, mnt sp.Tmount) bool {
	switch q {
	case "~any":
		return true
	case "~local":
		tip, _, err := mnt.TargetHostPort()
		if err != nil {
			return false
		}
		if tip == "" {
			tip = lip
		}
		db.DPrintf(db.MOUNT, "UnionMatch: %v tip %v lip %v\n", q, tip, lip)
		if tip == lip {
			return true
		}
		return false
	default:
		return true
	}
	return true
}
