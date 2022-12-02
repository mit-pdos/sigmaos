package spcodec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"

	"google.golang.org/protobuf/proto"

	"sigmaos/fcall"
	"sigmaos/frame"
	np "sigmaos/sigmap"
)

// Adopted from https://github.com/docker/go-p9p/encoding.go and Go's codecs

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
		case bool, uint8, uint16, uint32, uint64, np.Tseqno, fcall.Tsession, fcall.Tfcall, np.Ttag, np.Tfid, np.Tmode, np.Qtype, np.Tsize, np.Tpath, np.Tepoch, np.TQversion, np.Tperm, np.Tiounit, np.Toffset, np.Tlength, np.Tgid,
			*bool, *uint8, *uint16, *uint32, *uint64, *np.Tseqno, *fcall.Tsession, *fcall.Tfcall, *np.Ttag, *np.Tfid, *np.Tmode, *np.Qtype, *np.Tsize, *np.Tpath, *np.Tepoch, *np.TQversion, *np.Tperm, *np.Tiounit, *np.Toffset, *np.Tlength, *np.Tgid:
			if err := binary.Write(e.wr, binary.LittleEndian, v); err != nil {
				return err
			}
		case *[]byte:
			if err := e.encode(*v); err != nil {
				return err
			}
		case []byte:
			// XXX Bail out early to serialize separately
			if e.bailOut {
				return nil
			}
			if err := e.encode(uint32(len(v))); err != nil {
				return err
			}

			if err := binary.Write(e.wr, binary.LittleEndian, v); err != nil {
				return err
			}
		case string:
			if err := binary.Write(e.wr, binary.LittleEndian, uint16(len(v))); err != nil {
				return err
			}
			_, err := io.WriteString(e.wr, v)
			if err != nil {
				return err
			}
		case *string:
			if err := e.encode(*v); err != nil {
				return err
			}

		case []string:
			if err := e.encode(uint16(len(v))); err != nil {
				return err
			}

			for _, m := range v {
				if err := e.encode(m); err != nil {
					return err
				}
			}
		case *[]string:
			if err := e.encode(*v); err != nil {
				return err
			}
		case time.Time:
			if err := e.encode(uint32(v.Unix())); err != nil {
				return err
			}
		case *time.Time:
			if err := e.encode(*v); err != nil {
				return err
			}
		case np.Tqid:
			if err := e.encode(&v); err != nil {
				return err
			}
		case *np.Tqid:
			b, err := proto.Marshal(v)
			if err != nil {
				return err
			}
			if err := frame.PushToFrame(e.wr, b); err != nil {
				return err
			}
		case *np.Stat:
			b, err := proto.Marshal(v)
			if err != nil {
				return err
			}
			if err := frame.PushToFrame(e.wr, b); err != nil {
				return err
			}
		case []np.Tqid:
			if err := e.encode(uint16(len(v))); err != nil {
				return err
			}
			for _, m := range v {
				if err := e.encode(m); err != nil {
					return err
				}
			}
		case *[]np.Tqid:
			if err := e.encode(*v); err != nil {
				return err
			}
		case *[]fcall.Tsession:
			if err := e.encode(*v); err != nil {
				return err
			}
		case []fcall.Tsession:
			if err := e.encode(uint16(len(v))); err != nil {
				return err
			}
			for _, m := range v {
				if err := e.encode(m); err != nil {
					return err
				}
			}
		case np.Stat:
			if err := e.encode(&v); err != nil {
				return err
			}
		case np.FcallMsg:
			b, err := proto.Marshal(v.Fc)
			if err != nil {
				return err
			}
			if err := frame.PushToFrame(e.wr, b); err != nil {
				return err
			}
			switch fcall.Tfcall(v.Type()) {
			case fcall.TTwriteread, fcall.TRattach, fcall.TTwalk, fcall.TRwalk:
				b, err := proto.Marshal(v.Msg.(proto.Message))
				if err != nil {
					return err
				}
				if err := frame.PushToFrame(e.wr, b); err != nil {
					return err
				}
			default:
				if err := e.encode(v.Msg); err != nil {
					return err
				}
			}
		case *np.FcallMsg:
			if err := e.encode(*v); err != nil {
				return err
			}
		case np.Tmsg:
			elements, err := fields9p(v)
			if err != nil {
				return err
			}
			if err := e.encode(elements...); err != nil {
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
		case *bool, *uint8, *uint16, *uint32, *uint64, *np.Tseqno, *fcall.Tsession, *fcall.Tfcall, *np.Ttag, *np.Tfid, *np.Tmode, *np.Qtype, *np.Tsize, *np.Tpath, *np.Tepoch, *np.TQversion, *np.Tperm, *np.Tiounit, *np.Toffset, *np.Tlength, *np.Tgid:
			if err := binary.Read(d.rd, binary.LittleEndian, v); err != nil {
				return err
			}
		case *[]byte:
			var l uint32

			if err := d.decode(&l); err != nil {
				return err
			}

			if l > 0 {
				*v = make([]byte, int(l))
			}

			// XXX Switch to Reader.Read() rather than binary.Read() because the
			// binary package uses reflection, which imposes an extremely high
			// overhead that scaled with the size of the byte array. It's also much
			// more powerful than we need, since we're just serializing an array of
			// bytes, after all.
			if _, err := d.rd.Read(*v); err != nil && !(err == io.EOF && l == 0) {
				return err
			}

		case *string:
			var l uint16

			// implement string[s] encoding
			if err := d.decode(&l); err != nil {
				return err
			}

			b := make([]byte, l)

			n, err := io.ReadFull(d.rd, b)
			if err != nil {
				return err
			}

			if n != int(l) {
				return errors.New("bad string")
			}
			*v = string(b)
		case *[]string:
			var l uint16

			if err := d.decode(&l); err != nil {
				return err
			}
			elements := make([]interface{}, int(l))
			*v = make([]string, int(l))
			for i := range elements {
				elements[i] = &(*v)[i]
			}

			if err := d.decode(elements...); err != nil {
				return err
			}
		case *time.Time:
			var epoch uint32
			if err := d.decode(&epoch); err != nil {
				return err
			}

			*v = time.Unix(int64(epoch), 0).UTC()
		case *np.Tqid:
			b, err := frame.PopFromFrame(d.rd)
			if err != nil {
				return err
			}
			if err := proto.Unmarshal(b, v); err != nil {
				return err
			}
		case *np.Stat:
			b, err := frame.PopFromFrame(d.rd)
			if err != nil {
				return err
			}
			if err := proto.Unmarshal(b, v); err != nil {
				return err
			}
		case *[]np.Tqid:
			var l uint16

			if err := d.decode(&l); err != nil {
				return err
			}

			elements := make([]interface{}, int(l))
			*v = make([]np.Tqid, int(l))
			for i := range elements {
				elements[i] = &(*v)[i]
			}

			if err := d.decode(elements...); err != nil {
				return err
			}
		case *[]fcall.Tsession:
			var l uint16

			if err := d.decode(&l); err != nil {
				return err
			}
			elements := make([]interface{}, int(l))
			*v = make([]fcall.Tsession, int(l))
			for i := range elements {
				elements[i] = &(*v)[i]
			}

			if err := d.decode(elements...); err != nil {
				return err
			}
		case *np.FcallMsg:
			b, err := frame.PopFromFrame(d.rd)
			if err != nil {
				return err
			}
			if err := proto.Unmarshal(b, v.Fc); err != nil {
				return err
			}
			msg, error := newMsg(v.Type())
			if error != nil {
				return err
			}
			switch v.Type() {
			case fcall.TTwriteread, fcall.TRattach, fcall.TTwalk, fcall.TRwalk:
				b, err := frame.PopFromFrame(d.rd)
				if err != nil {
					return err
				}
				m := msg.(proto.Message)
				if err := proto.Unmarshal(b, m); err != nil {
					return err
				}
			default:
				if err := d.decode(msg); err != nil {
					return err
				}
			}
			v.Msg = msg
		case np.Tmsg:
			elements, err := fields9p(v)
			if err != nil {
				return err
			}

			if err := d.decode(elements...); err != nil {
				return err
			}
		default:
			errors.New("decode: unknown type")
		}
	}

	return nil
}

// fields9p lists the settable fields from a struct type for reading and
// writing. We are using a lot of reflection here for fairly static
// serialization but we can replace this in the future with generated code if
// performance is an issue.
func fields9p(v interface{}) ([]interface{}, *fcall.Err) {
	rv := reflect.Indirect(reflect.ValueOf(v))

	if rv.Kind() != reflect.Struct {
		return nil, fcall.MkErr(fcall.TErrBadFcall, "cannot extract fields from non-struct")
	}

	elements := make([]interface{}, 0, rv.NumField())
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)

		if !f.CanInterface() {
			// unexported field, skip it.
			continue
		}

		if f.CanAddr() {
			f = f.Addr()
		}

		elements = append(elements, f.Interface())
	}

	return elements, nil
}
