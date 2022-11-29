package realm

import (
	"path"

	np "sigmaos/sigmap"
)

const (
	REALM_CONFIG = "name/realm-config" // Store of realm configs
	NODED_CONFIG = "name/noded-config" // Store of noded configs
	REALM_NAMEDS = "name/realm-nameds" // Symlinks to realms' nameds
	REALM_FENCES = "name/realm-fences" // Fence around modifications to realm allocations.
	REALM_MGRS   = "name/realm-mgrs"   // Fence around modifications to realm allocations.
	NODEDS       = "nodeds"
)

// Path to this Realm's config file.
func RealmConfPath(realmId string) string {
	return path.Join(REALM_CONFIG, realmId)
}

// Path to this Noded's config file.
func NodedConfPath(nodedId string) string {
	return path.Join(NODED_CONFIG, nodedId)
}

// Path to this realm's namespace (symlink to its realm named).
func RealmPath(realmId string) string {
	return path.Join(REALM_NAMEDS, realmId)
}

// Path to the fence associated with this realm.
func realmFencePath(realmId string) string {
	return path.Join(REALM_FENCES, realmId)
}

// Path to this realm's realmmgr's fs.
func realmMgrPath(realmId string) string {
	return path.Join(REALM_MGRS, realmId)
}

// Path to this noded's fs.
func nodedPath(realmId string, nodedId string) string {
	return path.Join(realmMgrPath(realmId), NODEDS, nodedId)
}

func nodedCtlPath(realmId string, nodedId string) string {
	return path.Join(nodedPath(realmId, nodedId), np.RESOURCE_CTL)
}
