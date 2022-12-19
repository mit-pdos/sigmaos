package sessconnclnt

import (
	"sigmaos/fcall"
	sp "sigmaos/sigmap"
)

type Conn interface {
	Reset()                               // Indicates that an error has occurred, and the connection has been reset.
	CompleteRPC(*sp.FcallMsg, *fcall.Err) // Complete an RPC
}
