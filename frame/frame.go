package frame

import (
	"encoding/binary"
	"io"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
)

type Tframe []byte

func ReadFrame(rd io.Reader) (Tframe, *serr.Err) {
	var len uint32
	if err := binary.Read(rd, binary.LittleEndian, &len); err != nil {
		return nil, serr.NewErr(serr.TErrUnreachable, err)
	}
	db.DPrintf(db.FRAME, "ReadFrame %d\n", len)
	len = len - 4
	if len < 0 {
		return nil, serr.NewErr(serr.TErrUnreachable, "readMsg too short")
	}
	frame := make(Tframe, len)
	n, e := io.ReadFull(rd, frame)
	if n != int(len) {
		return nil, serr.NewErr(serr.TErrUnreachable, e)
	}
	return frame, nil
}

func ReadBuf(rd io.Reader) (Tframe, *serr.Err) {
	var len uint32
	if err := binary.Read(rd, binary.LittleEndian, &len); err != nil {
		return nil, serr.NewErr(serr.TErrUnreachable, err)
	}
	var buf Tframe
	if len > 0 {
		buf = make(Tframe, len)
		n, e := io.ReadFull(rd, buf)
		if n != int(len) {
			return nil, serr.NewErr(serr.TErrUnreachable, e)
		}
	}
	return buf, nil
}

func WriteFrame(wr io.Writer, frame Tframe) *serr.Err {
	l := uint32(len(frame) + 4) // +4 because that is how 9P wants it
	if err := binary.Write(wr, binary.LittleEndian, l); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return WriteRawBuffer(wr, frame)
}

func WriteRawBuffer(wr io.Writer, buf Tframe) *serr.Err {
	if n, err := wr.Write(buf); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	} else if n < len(buf) {
		return serr.NewErr(serr.TErrUnreachable, "writeRawBuffer too short")
	}
	return nil
}

func PushToFrame(wr io.Writer, b Tframe) error {
	if err := binary.Write(wr, binary.LittleEndian, uint32(len(b))); err != nil {
		return err
	}
	if err := binary.Write(wr, binary.LittleEndian, b); err != nil {
		return err
	}
	return nil
}

func PopFromFrame(rd io.Reader) (Tframe, error) {
	var l uint32
	if err := binary.Read(rd, binary.LittleEndian, &l); err != nil {
		if err != io.EOF {
			return nil, serr.NewErr(serr.TErrUnreachable, err.Error())
		}
		return nil, err
	}
	b := make(Tframe, int(l))
	if _, err := io.ReadFull(rd, b); err != nil && !(err == io.EOF && l == 0) {
		return nil, serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return b, nil
}

func WriteTag(tag sessp.Ttag, wr io.Writer) *serr.Err {
	t := uint32(tag)
	if err := binary.Write(wr, binary.LittleEndian, t); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return nil
}

func ReadTag(rdr io.Reader) (sessp.Ttag, *serr.Err) {
	var t uint32
	if err := binary.Read(rdr, binary.LittleEndian, &t); err != nil {
		return 0, serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return sessp.Ttag(t), nil
}

func WriteFrames(fs []Tframe, wrt io.Writer) *serr.Err {
	for _, f := range fs {
		db.DPrintf(db.FRAME, "writeFrame %v\n", f)
		if err := WriteFrame(wrt, f); err != nil {
			return err
		}
	}
	return nil
}

func ReadFrames(rdr io.Reader, nframe int) ([]Tframe, *serr.Err) {
	reply := make([]Tframe, nframe)
	for i := 0; i < nframe; i++ {
		f, err := ReadFrame(rdr)
		db.DPrintf(db.FRAME, "readFrame %v %v\n", f, err)
		if err != nil {
			return nil, err
		}
		reply[i] = f
	}
	return reply, nil
}

func ReadTagFrames(rdr io.Reader, nframe int) ([]Tframe, sessp.Ttag, *serr.Err) {
	tag, err := ReadTag(rdr)
	if err != nil {
		return nil, 0, err
	}
	db.DPrintf(db.TEST, "tag %v n %d\n", tag, nframe)
	fs, err := ReadFrames(rdr, nframe)
	if err != nil {
		return nil, 0, err
	}
	db.DPrintf(db.TEST, "tag %v fs %v\n", tag, fs)
	return fs, tag, err
}

func WriteTagFrames(fs []Tframe, tag sessp.Ttag, wrt io.Writer) *serr.Err {
	if err := WriteTag(tag, wrt); err != nil {
		return err
	}
	if err := WriteFrames(fs, wrt); err != nil {
		return err
	}
	return nil
}
