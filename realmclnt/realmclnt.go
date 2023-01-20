package realmclnt

import (
	"sync"

	"sigmaos/fslib"
	"sigmaos/kernelclnt"
	sp "sigmaos/sigmap"
)

type RealmMgr struct {
	mu     sync.Mutex
	fsl    *fslib.FsLib
	kclnt  *kernelclnt.KernelClnt
	nameds map[sp.Trealm][]string
}

func MakeRealmMgr(fsl *fslib.FsLib) *RealmMgr {
	rm := &RealmMgr{fsl: fsl}
	rm.nameds = make(map[sp.Trealm][]string)
	return rm
}

func (rm *RealmMgr) startNamed(realm sp.Trealm) error {
	if rm.kclnt == nil {
		kclnt, err := kernelclnt.MakeKernelClnt(rm.fsl, sp.BOOT+"~local/")
		if err != nil {
			return err
		}
		rm.kclnt = kclnt
	}
	return rm.kclnt.Boot("named", []string{"realm"})
}

func (rm *RealmMgr) lookupRealm(realm sp.Trealm) ([]string, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	nds, ok := rm.nameds[realm]
	if !ok {
		if err := rm.startNamed(realm); err != nil {
			return nil, err
		}
		rm.nameds[realm] = []string{}
		nds = []string{}
	}
	return nds, nil
}

// XXX return pn?
func (rm *RealmMgr) LookupNamed(realm sp.Trealm) ([]string, error) {
	nds, err := rm.lookupRealm(realm)
	if err != nil {
		return nil, err
	}
	return nds, nil
}
