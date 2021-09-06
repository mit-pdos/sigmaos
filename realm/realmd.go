package realm

import (
	"log"
)

type Realmd struct {
	Id      string
	RealmId string
}

func MakeRealmd() *Realmd {
	log.Fatalf("Error: MakeRealmd unimplemented")
	return nil
}

/*
 * Lifecycle of a realmd:
 * 1. Wait for assignment.
 * 2. On assignment, grab the realm lock.
 * 3. Check if this is the first machine in the realm.
 *   3a. If so, start a named & wait.
 *   3b. If not, get the realm's named address.
 * 4. Boot a system registerred under the realm's named.
 * 5. Release the realm lock.
 */
