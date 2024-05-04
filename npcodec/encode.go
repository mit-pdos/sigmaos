package npcodec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"

	db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/spcodec"
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
		case bool, uint8, uint16, uint32, uint64, sessp.Tfcall, sessp.Ttag, np.Qtype9P, np.Tpath, np.TQversion, np.Tperm, np.Tlength,
			*bool, *uint8, *uint16, *uint32, *uint64:
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
		case np.Tqid9P:
			if err := e.encode(v.Type, v.Version, v.Path); err != nil {
				return err
			}
		case sp.Tqid:
			t := np.Qtype9P(v.Type)
			if err := e.encode(t, v.Version, v.Path); err != nil {
				return err
			}
		case *sp.Tqid:
			if err := e.encode(*v); err != nil {
				return err
			}
		case **sp.Tqid:
			if err := e.encode(**v); err != nil {
				return err
			}
		case []*sp.Tqid:
			if err := e.encode(uint16(len(v))); err != nil {
				return err
			}

			for _, m := range v {
				if err := e.encode(m); err != nil {
					return err
				}
			}
		case *[]*sp.Tqid:
			if err := e.encode(*v); err != nil {
				return err
			}
		case np.Stat9P:
			elements, err := fields9p(v)
			if err != nil {
				return err
			}
			sz := uint16(sizeNp(elements...)) // Stat sz
			if err := e.encode(sz); err != nil {
				return err
			}
			if err := e.encode(elements...); err != nil {
				return err
			}
		case *np.Stat9P:
			if err := e.encode(*v); err != nil {
				return err
			}
		case Fcall9P:
			msg := v.Msg
			if v.Type == sessp.TRstat {
				sprstat := msg.(*sp.Rrstat)
				npst := Sp2NpStat(sprstat.Stat)
				msg = &np.Rstat9P{0, *npst}
			}
			if err := e.encode(v.Type, v.Tag, msg); err != nil {
				return err
			}
		case *Fcall9P:
			if err := e.encode(*v); err != nil {
				return err
			}
		case sessp.Tmsg:
			elements, err := fields9p(v)
			if err != nil {
				return err
			}
			if err := e.encode(elements...); err != nil {
				return err
			}
		default:
			return errors.New(fmt.Sprintf("Encode unknown type: %T", v))
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
		case *bool, *uint8, *uint16, *uint32, *uint64, *sessp.Tfcall, *sessp.Ttag, *np.Tfid, *np.Toffset, *np.Tsize, *np.Tmode9P, *np.Tperm, *np.Tlength, *np.Tpath, *np.TQversion, *np.Qtype9P:
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
		case *np.Tqid9P:
			if err := d.decode(&v.Type, &v.Version, &v.Path); err != nil {
				return err
			}
		case *np.Stat9P:
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
		case *Fcall9P:
			if err := d.decode(&v.Type, &v.Tag); err != nil {
				return err
			}
			var msg sessp.Tmsg
			// XXX maybe also TTflush
			if v.Type == sessp.TTattach9P {
				msg = &np.Tattach9P{}
			} else if v.Type == sessp.TTread {
				msg = &np.Tread{}
			} else if v.Type == sessp.TTwrite {
				msg = &np.Twrite{}
			} else if v.Type == sessp.TTopen9P {
				msg = &np.Topen9P{}
			} else if v.Type == sessp.TTcreate9P {
				msg = &np.Tcreate9P{}
			} else if v.Type == sessp.TTwstat9P {
				msg = &np.Twstat9P{}
			} else if v.Type == sessp.TTremove9P {
				msg = &np.Tremove9P{}
			} else {
				m, err := spcodec.NewMsg(v.Type)
				if err != nil {
					return err
				}
				msg = m
			}
			if err := d.decode(msg); err != nil {
				return err
			}
			v.Msg = msg
		case sessp.Tmsg:
			elements, err := fields9p(v)
			if err != nil {
				return err
			}

			if err := d.decode(elements...); err != nil {
				return err
			}
		default:
			return fmt.Errorf("Decode unknown type: %T", v)
		}
	}

	return nil
}

// SizeNp calculates the projected size of the values in vs when encoded into
// 9p binary protocol.  Used only for Stat
// encoding.
func sizeNp(vs ...interface{}) uint64 {
	var s uint64
	for _, v := range vs {
		if v == nil {
			continue
		}
		switch v := v.(type) {
		case uint16, uint32, np.Tpath, np.TQversion, np.Tperm, np.Tlength, np.Qtype9P:
			s += uint64(binary.Size(v))
		case sp.Tqid:
			s += sizeNp(v.Type, v.Version, v.Path)
		case np.Tqid9P:
			s += sizeNp(v.Type, v.Version, v.Path)
		case string:
			s += uint64(binary.Size(uint16(0)) + len(v))
		case np.Stat9P:
			elements, err := fields9p(v)
			if err != nil {
				db.DFatalf("Stat %v", err)
			}
			s += sizeNp(elements...) + sizeNp(uint16(0))
		default:
			db.DFatalf("sizeNp: Unknown type %T", v)
		}
	}
	return s
}

// fields9p lists the settable fields from a struct type for reading and
// writing. We are using a lot of reflection here for fairly static
// serialization but we can replace this in the future with generated code if
// performance is an issue.
func fields9p(v interface{}) ([]interface{}, *serr.Err) {
	rv := reflect.Indirect(reflect.ValueOf(v))

	if rv.Kind() != reflect.Struct {
		return nil, serr.NewErr(serr.TErrBadFcall, "cannot extract fields from non-struct")
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
