package sessconnclnt

import (
	np "sigmaos/ninep"
)

type Conn interface {
	Reset()                         // Indicates that an error has occurred, and the connection has been reset.
	CompleteRPC(*np.Fcall, *np.Err) // Complete an RPC
}
