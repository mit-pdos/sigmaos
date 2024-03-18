package frame

import (
	"encoding/binary"
	"io"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
)

func ReadFrame(rd io.Reader) (sessp.Tframe, *serr.Err) {
	var f sessp.Tframe = nil
	err := ReadFrameInto(rd, &f)
	return f, err
}

func ReadFrameInto(rd io.Reader, frame *sessp.Tframe) *serr.Err {
	var nbyte uint32
	if err := binary.Read(rd, binary.LittleEndian, &nbyte); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err)
	}
	db.DPrintf(db.FRAME, "ReadFrameInto %d", nbyte)
	nbyte = nbyte - 4
	if nbyte < 0 {
		return serr.NewErr(serr.TErrUnreachable, "readMsg too short")
	}
	// If no frame to read into was specified, allocate one
	if *frame == nil {
		*frame = make(sessp.Tframe, nbyte)
	} else {
	}
	if nbyte > uint32(len(*frame)) {
		db.DFatalf("Output buf too smal: %v < %v", len(*frame), nbyte)
	}
	// Only read the first nbyte bytes
	buf := (*frame)[:nbyte]
	n, e := io.ReadFull(rd, buf)
	if n != int(nbyte) {
		return serr.NewErr(serr.TErrUnreachable, e)
	}
	return nil
}

func ReadNFramesInto(rd io.Reader, iov sessp.IoVec) *serr.Err {
	for i := 0; i < len(iov); i++ {
		err := ReadFrameInto(rd, &iov[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func GetNFrames(rd io.Reader) (uint32, *serr.Err) {
	var len uint32
	if err := binary.Read(rd, binary.LittleEndian, &len); err != nil {
		return 0, serr.NewErr(serr.TErrUnreachable, err)
	}
	return len, nil
}

func ReadFrames(rd io.Reader) (sessp.IoVec, *serr.Err) {
	len, err := GetNFrames(rd)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.FRAME, "ReadFrames %d\n", len)
	len = len - 4
	if len < 0 {
		return nil, serr.NewErr(serr.TErrUnreachable, "ReadFrames too short")
	}
	iov := make(sessp.IoVec, len)
	for i := 0; i < int(len); i++ {
		err := ReadFrameInto(rd, &iov[i])
		if err != nil {
			return nil, err
		}
	}
	return iov, nil
}

func WriteFrame(wr io.Writer, frame sessp.Tframe) *serr.Err {
	l := uint32(len(frame) + 4) // +4 because that is how 9P wants it
	if err := binary.Write(wr, binary.LittleEndian, l); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return WriteRawBuffer(wr, frame)
}

func WriteFrames(wr io.Writer, iov sessp.IoVec) *serr.Err {
	l := uint32(len(iov) + 4) // +4 because that is how 9P wants it
	if err := binary.Write(wr, binary.LittleEndian, l); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	for _, f := range iov {
		if err := WriteFrame(wr, f); err != nil {
			return err
		}
	}
	return nil
}

func WriteRawBuffer(wr io.Writer, buf sessp.Tframe) *serr.Err {
	if n, err := wr.Write(buf); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	} else if n < len(buf) {
		return serr.NewErr(serr.TErrUnreachable, "writeRawBuffer too short")
	}
	return nil
}

func PushToFrame(wr io.Writer, b sessp.Tframe) error {
	if err := binary.Write(wr, binary.LittleEndian, uint32(len(b))); err != nil {
		return err
	}
	if err := binary.Write(wr, binary.LittleEndian, b); err != nil {
		return err
	}
	return nil
}

func PopFromFrame(rd io.Reader) (sessp.Tframe, error) {
	var l uint32
	if err := binary.Read(rd, binary.LittleEndian, &l); err != nil {
		if err != io.EOF {
			return nil, serr.NewErr(serr.TErrUnreachable, err.Error())
		}
		return nil, err
	}
	db.DPrintf(db.ALWAYS, "Read %v byte frame", l)
	b := make(sessp.Tframe, int(l))
	if _, err := io.ReadFull(rd, b); err != nil && !(err == io.EOF && l == 0) {
		return nil, serr.NewErr(serr.TErrUnreachable, err.Error())
	} else {
		db.DPrintf(db.ALWAYS, "Read %v byte frame err %v", l, err)
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
