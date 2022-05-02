package realm

import (
	"runtime/debug"

	db "ulambda/debug"
	"ulambda/electclnt"
)

func lockRealm(ec *electclnt.ElectClnt, realmId string) {
	if err := ec.AcquireLeadership([]byte("sigmamgr")); err != nil {
		db.DFatalf("%v error SigmaResourceMgr acquire leadership: %v", string(debug.Stack()), err)
	}
}

func unlockRealm(ec *electclnt.ElectClnt, realmId string) {
	if err := ec.ReleaseLeadership(); err != nil {
		db.DFatalf("%v error SigmaResourceMgr release leadership: %v", string(debug.Stack()), err)
	}
}
