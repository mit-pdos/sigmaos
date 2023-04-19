package sessconnclnt

import (
	"sigmaos/serr"
	"sigmaos/sessp"
)

type Conn interface {
	Reset()                                              // Indicates that an error has occurred, and the connection has been reset.
	CompleteRPC(sessp.Tseqno, []byte, []byte, *serr.Err) // Complete an RPC
}
