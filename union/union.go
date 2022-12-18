package union

import (
	"sigmaos/fidclnt"
	sp "sigmaos/sigmap"
)

func UnionMatch(q, name string) bool {
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
