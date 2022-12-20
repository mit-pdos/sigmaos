package sessconnclnt

import (
	"sigmaos/sessp"
)

type Conn interface {
	Reset()                                  // Indicates that an error has occurred, and the connection has been reset.
	CompleteRPC(*sessp.FcallMsg, *sessp.Err) // Complete an RPC
}
