package sessconnclnt

import (
	"sigmaos/fcall"
)

type Conn interface {
	Reset()                                  // Indicates that an error has occurred, and the connection has been reset.
	CompleteRPC(*fcall.FcallMsg, *fcall.Err) // Complete an RPC
}
