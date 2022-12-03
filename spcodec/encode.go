package spcodec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"

	"google.golang.org/protobuf/proto"

	"sigmaos/frame"
	np "sigmaos/sigmap"
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

func unmarshal(data []byte, v interface{}) error {
	return unmarshalReader(bytes.NewReader(data), v)
}

func unmarshalReader(rdr io.Reader, v interface{}) error {
	dec := &decoder{rdr}
	return dec.decode(v)
}

type encoder struct {
	bailOut bool // Optionally bail out when marshalling buffers
	wr      io.Writer
}

func (e *encoder) encode(vs ...interface{}) error {
	for _, v := range vs {
		switch v := v.(type) {
		case np.FcallMsg:
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
		case *np.FcallMsg:
			if err := e.encode(*v); err != nil {
				return err
			}
		default:
			return errors.New(fmt.Sprintf("Encode: unknown type: %v", reflect.TypeOf(v)))
		}
	}

	return nil
}

type decoder struct {
	rd io.Reader
}

func (d *decoder) decode(vs ...interface{}) error {
	for _, v := range vs {
		switch v := v.(type) {
		case *np.FcallMsg:
			b, err := frame.PopFromFrame(d.rd)
			if err != nil {
				return err
			}
			if err := proto.Unmarshal(b, v.Fc); err != nil {
				return err
			}
			msg, error := NewMsg(v.Type())
			if error != nil {
				return err
			}
			b, err = frame.PopFromFrame(d.rd)
			if err != nil {
				return err
			}
			m := msg.(proto.Message)
			if err := proto.Unmarshal(b, m); err != nil {
				return err
			}
			v.Msg = msg
		default:
			return fmt.Errorf("decode: unknown type %T", v)
		}
	}

	return nil
}
