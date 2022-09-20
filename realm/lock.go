package realm

import (
	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/electclnt"
)

func lockRealm(ec *electclnt.ElectClnt, realmId string) {
	db.DPrintf("REALMLOCK", "try lock realm %v", realmId)
	if err := ec.AcquireLeadership([]byte("sigmamgr")); err != nil {
		db.DFatalf("%v error SigmaResourceMgr acquire leadership: %v", string(debug.Stack()), err)
	}
	db.DPrintf("REALMLOCK", "acquire lock realm %v", realmId)
}

func unlockRealm(ec *electclnt.ElectClnt, realmId string) {
	db.DPrintf("REALMLOCK", "unlock realm %v", realmId)
	if err := ec.ReleaseLeadership(); err != nil {
		db.DFatalf("%v error SigmaResourceMgr release leadership: %v", string(debug.Stack()), err)
	}
}
