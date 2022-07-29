package realm

import (
	"path"
)

const (
	REALM_CONFIG = "name/realm-config" // Store of realm configs
	NODED_CONFIG = "name/noded-config" // Store of noded configs
)

func RealmConfPath(realmId string) string {
	return path.Join(REALM_CONFIG, realmId)
}

func NodedConfPath(nodedId string) string {
	return path.Join(NODED_CONFIG, nodedId)
}
