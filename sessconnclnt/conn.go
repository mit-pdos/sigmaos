package sessconnclnt

import (
	"sigmaos/fcall"
	np "sigmaos/sigmap"
)

type Conn interface {
	Reset()                               // Indicates that an error has occurred, and the connection has been reset.
	CompleteRPC(*np.FcallMsg, *fcall.Err) // Complete an RPC
}
