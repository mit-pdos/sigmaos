package fsobjsrv

import (
	"encoding/json"
	"log"
)

func (fos *FsObjSrv) Snapshot() []byte {
	// TODO
	b, err := json.Marshal(nil)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding fsobjsrv: %v", err)
	}
	return b
}
