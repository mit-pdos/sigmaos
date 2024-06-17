package protsrv

import (
	"fmt"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

// Allow attaches from all principals to all paths
func AttachAllowAllToAll(p *sp.Tprincipal, pn string) error {
	db.DPrintf(db.PROTSRV, "Allow attach from %v to %v", p, pn)
	return nil
}

// Allow attaches from all principals to select paths
func AttachAllowAllPrincipalsSelectPaths(pns []string) AttachAuthF {
	allowed := make(map[string]bool)
	for _, d := range pns {
		allowed[d] = true
	}
	return func(p *sp.Tprincipal, pn string) error {
		if p.GetRealm() == sp.ROOTREALM {
			// Always allow attaches from root realm to any path
			db.DPrintf(db.PROTSRV, "Allow attach from %v to \"%v\"", p, pn)
			return nil
		}
		if allowed[pn] {
			db.DPrintf(db.PROTSRV, "Allow attach from %v to \"%v\"", p, pn)
			return nil
		}
		// If path isn't in specified allowed paths, reject connection attempt
		db.DPrintf(db.PROTSRV, "Unauthorized attach from %v to \"%v\"", p, pn)
		return fmt.Errorf("Unauthorized attach from %v to %v", p, pn)
	}
}
