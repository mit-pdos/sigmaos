package realm

import (
	"path"
)

const (
	REALM_CONFIG = "name/realm-config" // Store of realm configs
	NODED_CONFIG = "name/noded-config" // Store of noded configs
	REALM_NAMEDS = "name/realm-nameds" // Symlinks to realms' nameds
	REALM_FENCES = "name/realm-fences" // Fence around modifications to realm allocations.
	_REALM_MGRS  = "name/realm-mgrs"   // Fence around modifications to realm allocations.
)

func RealmConfPath(realmId string) string {
	return path.Join(REALM_CONFIG, realmId)
}

func NodedConfPath(nodedId string) string {
	return path.Join(NODED_CONFIG, nodedId)
}

func RealmPath(realmId string) string {
	return path.Join(REALM_NAMEDS, realmId)
}

func realmFencePath(realmId string) string {
	return path.Join(REALM_FENCES, realmId)
}

func realmMgrPath(realmId string) string {
	return path.Join(_REALM_MGRS, realmId)
}
