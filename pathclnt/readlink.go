package pathclnt

import (
	"errors"

	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/reader"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type rdr struct {
	*fidclnt.FidClnt
	fid sp.Tfid
	f   *sp.Tfence
}

func newRdr(fdc *fidclnt.FidClnt, fid sp.Tfid, f *sp.Tfence) *rdr {
	return &rdr{fdc, fid, f}
}

func (rd *rdr) Close() error {
	return rd.FidClnt.Clunk(rd.fid)
}

func (rd *rdr) Read(o sp.Toffset, b []byte) (int, error) {
	n, err := rd.ReadF(rd.fid, o, b, rd.f)
	if err != nil {
		return int(n), err
	}
	return int(n), nil
}

func (pathc *PathClnt) readlink(fid sp.Tfid) ([]byte, *serr.Err) {
	db.DPrintf(db.PATHCLNT, "readlink %v", fid)
	qid := pathc.Qid(fid)
	if qid.Ttype()&sp.QTSYMLINK == 0 {
		return nil, serr.NewErr(serr.TErrNotSymlink, qid.Type)
	}
	_, err := pathc.FidClnt.Open(fid, sp.OREAD)
	if err != nil {
		return nil, err
	}
	rdr := reader.NewReader(newRdr(pathc.FidClnt, fid, sp.NullFence()), "")
	b, r := rdr.GetData()
	if errors.As(r, &err) {
		return nil, err
	}
	return b, nil
}
