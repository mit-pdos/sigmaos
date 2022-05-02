package realm

import (
	"runtime/debug"

	db "ulambda/debug"
	"ulambda/electclnt"
)

func lockRealm(ec *electclnt.ElectClnt, realmId string) {
	db.DPrintf(db.ALWAYS, "try lock realm %v", realmId)
	if err := ec.AcquireLeadership([]byte("sigmamgr")); err != nil {
		db.DFatalf("%v error SigmaResourceMgr acquire leadership: %v", string(debug.Stack()), err)
	}
	db.DPrintf(db.ALWAYS, "acquire lock realm %v", realmId)
}

func unlockRealm(ec *electclnt.ElectClnt, realmId string) {
	db.DPrintf(db.ALWAYS, "unlock realm %v", realmId)
	if err := ec.ReleaseLeadership(); err != nil {
		db.DFatalf("%v error SigmaResourceMgr release leadership: %v", string(debug.Stack()), err)
	}
}
