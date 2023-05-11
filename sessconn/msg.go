package sessconn

import (
	"sigmaos/sessp"
	"sigmaos/spcodec"
)

// A partially marshaled message, ready to be sent on a session.
type PartMarshaledMsg struct {
	Fcm          *sessp.FcallMsg
	MarshaledFcm []byte
}

func MakePartMarshaledMsg(fcm *sessp.FcallMsg) *PartMarshaledMsg {
	return &PartMarshaledMsg{
		fcm,
		spcodec.MarshalFcallWithoutData(fcm),
	}
}
