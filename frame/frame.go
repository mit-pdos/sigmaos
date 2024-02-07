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

func WriteSeqno(seqno sessp.Tseqno, wr io.Writer) *serr.Err {
	sn := uint64(seqno)
	if err := binary.Write(wr, binary.LittleEndian, sn); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return nil
}

func ReadSeqno(rdr io.Reader) (sessp.Tseqno, *serr.Err) {
	var sn uint64
	if err := binary.Read(rdr, binary.LittleEndian, &sn); err != nil {
		return 0, serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return sessp.Tseqno(sn), nil
}
