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
}

func newRdr(fdc *fidclnt.FidClnt, fid sp.Tfid) *rdr {
	return &rdr{fdc, fid}
}

func (rd *rdr) Close() error {
	return rd.FidClnt.Clunk(rd.fid)
}

func (rd *rdr) Read(o sp.Toffset, sz sp.Tsize) ([]byte, error) {
	b, err := rd.ReadF(rd.fid, o, sz)
	if err != nil {
		return b, err
	}
	return b, nil
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
	rdr := reader.NewReader(newRdr(pathc.FidClnt, fid), "")
	b, r := rdr.GetDataErr()
	if errors.As(r, &err) {
		return nil, err
	}
	return b, nil
}
