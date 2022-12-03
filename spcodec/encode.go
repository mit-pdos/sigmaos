package spcodec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"

	"google.golang.org/protobuf/proto"

	"sigmaos/frame"
	sp "sigmaos/sigmap"
)

func marshal(v interface{}) ([]byte, error) {
	return marshal1(false, v)
}

func marshal1(bailOut bool, v interface{}) ([]byte, error) {
	var b bytes.Buffer
	enc := &encoder{bailOut, &b}
	if err := enc.encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

type encoder struct {
	bailOut bool // Optionally bail out when marshalling buffers
	wr      io.Writer
}

func (e *encoder) encode(vs ...interface{}) error {
	for _, v := range vs {
		switch v := v.(type) {
		case sp.FcallMsg:
			b, err := proto.Marshal(v.Fc)
			if err != nil {
				return err
			}
			if err := frame.PushToFrame(e.wr, b); err != nil {
				return err
			}
			b, err = proto.Marshal(v.Msg.(proto.Message))
			if err != nil {
				return err
			}
			if err := frame.PushToFrame(e.wr, b); err != nil {
				return err
			}
		case *sp.FcallMsg:
			if err := e.encode(*v); err != nil {
				return err
			}
		default:
			return errors.New(fmt.Sprintf("Encode: unknown type: %v", reflect.TypeOf(v)))
		}
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
