package sessconn

// XXX delete

import (
	"sigmaos/sessp"
	//"sigmaos/spcodec"
)

// A partially marshaled message, ready to be sent on a session.
type PartMarshaledMsg struct {
	Fcm          *sessp.FcallMsg
	MarshaledFcm []byte
}

func NewPartMarshaledMsg(fcm *sessp.FcallMsg) *PartMarshaledMsg {
	return &PartMarshaledMsg{
		Fcm: fcm,
		//spcodec.MarshalFcallWithoutData(fcm),
	}
}
