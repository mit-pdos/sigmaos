package sessconnclnt

import (
	"sigmaos/sessp"
    "sigmaos/serr"
)

type Conn interface {
	Reset()                                  // Indicates that an error has occurred, and the connection has been reset.
	CompleteRPC(*sessp.FcallMsg, *serr.Err) // Complete an RPC
}
