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

	db "sigmaos/debug"
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
		case bool, uint8, uint16, uint32, uint64, np.Tseqno, np.Tsession, np.Tfcall, np.Ttag, np.Tfid, np.Tmode, np.Qtype, np.Tsize, np.Tpath, np.Tepoch, np.TQversion, np.Tperm, np.Tiounit, np.Toffset, np.Tlength, np.Tgid,
			*bool, *uint8, *uint16, *uint32, *uint64, *np.Tseqno, *np.Tsession, *np.Tfcall, *np.Ttag, *np.Tfid, *np.Tmode, *np.Qtype, *np.Tsize, *np.Tpath, *np.Tepoch, *np.TQversion, *np.Tperm, *np.Tiounit, *np.Toffset, *np.Tlength, *np.Tgid:
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
			if err := e.encode(v.Type, v.Version, v.Path); err != nil {
				return err
			}
		case *np.Tqid:
			if err := e.encode(*v); err != nil {
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
		case *[]np.Tsession:
			if err := e.encode(*v); err != nil {
				return err
			}
		case []np.Tsession:
			if err := e.encode(uint16(len(v))); err != nil {
				return err
			}
			for _, m := range v {
				if err := e.encode(m); err != nil {
					return err
				}
			}
		case np.Stat:
			elements, err := fields9p(v)
			if err != nil {
				return err
			}
			sz := uint16(SizeNp(elements...)) // Stat sz
			if err := e.encode(sz); err != nil {
				return err
			}

			if err := e.encode(elements...); err != nil {
				return err
			}
		case *np.Stat:
			if err := e.encode(*v); err != nil {
				return err
			}
		case []np.Stat:
			if err := e.encode(uint16(len(v))); err != nil {
				return err
			}
			for _, m := range v {
				if err := e.encode(m); err != nil {
					return err
				}
			}
		case *[]np.Stat:
			if err := e.encode(*v); err != nil {
				return err
			}
		case np.FcallWireCompat:
			if err := e.encode(v.Type, v.Tag, v.Msg); err != nil {
				return err
			}
		case *np.FcallWireCompat:
			if err := e.encode(*v); err != nil {
				return err
			}
		case np.FcallMsg:
			b, err := proto.Marshal(v.Fc)
			if err != nil {
				return err
			}
			if err := e.encode(uint32(len(b))); err != nil {
				return err
			}
			if err := binary.Write(e.wr, binary.LittleEndian, b); err != nil {
				return err
			}
			switch np.Tfcall(v.Fc.Type) {
			case np.TTwriteread:
				b, err := proto.Marshal(v.Msg.(proto.Message))
				if err != nil {
					return err
				}
				if err := e.encode(uint32(len(b))); err != nil {
					return err
				}
				if err := binary.Write(e.wr, binary.LittleEndian, b); err != nil {
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
			return errors.New(fmt.Sprintf("Unknown type: %v", reflect.TypeOf(v)))
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
		case *bool, *uint8, *uint16, *uint32, *uint64, *np.Tseqno, *np.Tsession, *np.Tfcall, *np.Ttag, *np.Tfid, *np.Tmode, *np.Qtype, *np.Tsize, *np.Tpath, *np.Tepoch, *np.TQversion, *np.Tperm, *np.Tiounit, *np.Toffset, *np.Tlength, *np.Tgid:
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
			if err := d.decode(&v.Type, &v.Version, &v.Path); err != nil {
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
		case *[]np.Tsession:
			var l uint16

			if err := d.decode(&l); err != nil {
				return err
			}
			elements := make([]interface{}, int(l))
			*v = make([]np.Tsession, int(l))
			for i := range elements {
				elements[i] = &(*v)[i]
			}

			if err := d.decode(elements...); err != nil {
				return err
			}
		case *np.Stat:
			var l uint16

			if err := d.decode(&l); err != nil {
				return err
			}

			b := make([]byte, l)
			if _, err := io.ReadFull(d.rd, b); err != nil {
				return err
			}

			elements, err := fields9p(v)
			if err != nil {
				return err
			}

			dec := &decoder{bytes.NewReader(b)}

			if err := dec.decode(elements...); err != nil {
				return err
			}
		case *np.FcallWireCompat:
			if err := d.decode(&v.Type, &v.Tag); err != nil {
				return err
			}
			msg, err := newMsg(v.Type)
			if err != nil {
				return err
			}
			if err := d.decode(msg); err != nil {
				return err
			}
			v.Msg = msg
		case *np.FcallMsg:
			var l uint32
			if err := d.decode(&l); err != nil {
				return err
			}
			b := make([]byte, int(l))
			if _, err := d.rd.Read(b); err != nil && !(err == io.EOF && l == 0) {
				return err
			}
			if err := proto.Unmarshal(b, v.Fc); err != nil {
				return err
			}
			msg, err := newMsg(np.Tfcall(v.Fc.Type))
			if err != nil {
				return err
			}
			switch np.Tfcall(v.Fc.Type) {
			case np.TTwriteread:
				var l uint32
				if err := d.decode(&l); err != nil {
					return err
				}
				b := make([]byte, int(l))
				if _, err := d.rd.Read(b); err != nil && !(err == io.EOF && l == 0) {
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
			errors.New("unknown type")
		}
	}

	return nil
}

// SizeNp calculates the projected size of the values in vs when encoded into
// 9p binary protocol. If an element or elements are not valid for 9p encoded,
// the value 0 will be used for the size. The error will be detected when
// encoding.
func SizeNp(vs ...interface{}) uint32 {
	var s uint32
	for _, v := range vs {
		if v == nil {
			continue
		}

		switch v := v.(type) {
		case bool, uint8, uint16, uint32, uint64, np.Tseqno, np.Tsession, np.Tfcall, np.Ttag, np.Tfid, np.Tmode, np.Qtype, np.Tsize, np.Tpath, np.Tepoch, np.TQversion, np.Tperm, np.Tiounit, np.Toffset, np.Tlength, np.Tgid,
			*bool, *uint8, *uint16, *uint32, *uint64, *np.Tseqno, *np.Tsession, *np.Tfcall, *np.Ttag, *np.Tfid, *np.Tmode, *np.Qtype, *np.Tsize, *np.Tpath, *np.Tepoch, *np.TQversion, *np.Tperm, *np.Tiounit, *np.Toffset, *np.Tlength, *np.Tgid:
			s += uint32(binary.Size(v))
		case []byte:
			s += uint32(binary.Size(uint32(0)) + len(v))
		case *[]byte:
			s += SizeNp(uint32(0), *v)
		case string:
			s += uint32(binary.Size(uint16(0)) + len(v))
		case *string:
			s += SizeNp(*v)
		case []string:
			s += SizeNp(uint16(0))

			for _, sv := range v {
				s += SizeNp(sv)
			}
		case *[]string:
			s += SizeNp(*v)
		case np.Tqid:
			s += SizeNp(v.Type, v.Version, v.Path)
		case *np.Tqid:
			s += SizeNp(*v)
		case []np.Tqid:
			s += SizeNp(uint16(0))
			elements := make([]interface{}, len(v))
			for i := range elements {
				elements[i] = &v[i]
			}
			s += SizeNp(elements...)
		case *[]np.Tqid:
			s += SizeNp(*v)
		case np.Stat:
			elements, err := fields9p(v)
			if err != nil {
				db.DFatalf("Stat %v", err)
			}
			s += SizeNp(elements...) + SizeNp(uint16(0))
		case *np.Stat:
			s += SizeNp(*v)
		case []np.Stat:
			elements := make([]interface{}, len(v))
			for i := range elements {
				elements[i] = &v[i]
			}
			s += SizeNp(elements...)
		case *[]np.Stat:
			s += SizeNp(*v)
		case np.FcallWireCompat:
			s += SizeNp(v.Type, v.Tag, v.Msg)
		case *np.FcallWireCompat:
			s += SizeNp(*v)
		case np.Fcall:
			s += SizeNp(v.Type, v.Tag, v.Session, v.Seqno, *v.Received, *v.Fence)
		case *np.Fcall:
			s += SizeNp(*v)
		case np.Tmsg:
			// walk the fields of the message to get the total size. we just
			// use the field order from the message struct. We may add tag
			// ignoring if needed.
			elements, err := fields9p(v)
			if err != nil {
				db.DFatalf("Tmsg %v", err)
			}

			s += SizeNp(elements...)
		default:
			db.DFatalf("Unknown type")
		}
	}

	return s
}

// fields9p lists the settable fields from a struct type for reading and
// writing. We are using a lot of reflection here for fairly static
// serialization but we can replace this in the future with generated code if
// performance is an issue.
func fields9p(v interface{}) ([]interface{}, *np.Err) {
	rv := reflect.Indirect(reflect.ValueOf(v))

	if rv.Kind() != reflect.Struct {
		return nil, np.MkErr(np.TErrBadFcall, "cannot extract fields from non-struct")
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
