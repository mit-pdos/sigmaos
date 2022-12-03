package spcodec

import (
	"io"

	"google.golang.org/protobuf/proto"

	"sigmaos/frame"
	sp "sigmaos/sigmap"
)

func encode(wr io.Writer, fcm *sp.FcallMsg) error {
	b, err := proto.Marshal(fcm.Fc)
	if err != nil {
		return err
	}
	if err := frame.PushToFrame(wr, b); err != nil {
		return err
	}
	b, err = proto.Marshal(fcm.Msg.(proto.Message))
	if err != nil {
		return err
	}
	if err := frame.PushToFrame(wr, b); err != nil {
		return err
	}
	return nil
}

func decode(rdr io.Reader, fcm *sp.FcallMsg) error {
	b, err := frame.PopFromFrame(rdr)
	if err != nil {
		return err
	}
	if err := proto.Unmarshal(b, fcm.Fc); err != nil {
		return err
	}
	msg, error := NewMsg(fcm.Type())
	if error != nil {
		return err
	}
	b, err = frame.PopFromFrame(rdr)
	if err != nil {
		return err
	}
	m := msg.(proto.Message)
	if err := proto.Unmarshal(b, m); err != nil {
		return err
	}
	fcm.Msg = msg
	return nil
}
