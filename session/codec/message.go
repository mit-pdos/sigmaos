package codec

import (
	"fmt"

	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
)

func NewMsg(typ sessp.Tfcall) (sessp.Tmsg, *serr.Err) {
	switch typ {
	case sessp.TTversion:
		return &sp.Tversion{}, nil
	case sessp.TRversion:
		return &sp.Rversion{}, nil
	case sessp.TTauth:
		return &sp.Tauth{}, nil
	case sessp.TRauth:
		return &sp.Rauth{}, nil
	case sessp.TTattach:
		return &sp.Tattach{}, nil
	case sessp.TRattach:
		return &sp.Rattach{}, nil
	case sessp.TRerror:
		return &sp.Rerror{}, nil
	case sessp.TTwalk:
		return &sp.Twalk{}, nil
	case sessp.TRwalk:
		return &sp.Rwalk{}, nil
	case sessp.TTopen:
		return &sp.Topen{}, nil
	case sessp.TRopen:
		return &sp.Ropen{}, nil
	case sessp.TTcreate:
		return &sp.Tcreate{}, nil
	case sessp.TRcreate:
		return &sp.Rcreate{}, nil
	case sessp.TRread:
		return &sp.Rread{}, nil
	case sessp.TRwrite:
		return &sp.Rwrite{}, nil
	case sessp.TTclunk:
		return &sp.Tclunk{}, nil
	case sessp.TRclunk:
		return &sp.Rclunk{}, nil
	case sessp.TTremove:
		return &sp.Tremove{}, nil
	case sessp.TRremove:
		return &sp.Rremove{}, nil
	case sessp.TTstat:
		return &sp.Trstat{}, nil
	case sessp.TRstat:
		return &sp.Rrstat{}, nil
	case sessp.TTwstat:
		return &sp.Twstat{}, nil
	case sessp.TRwstat:
		return &sp.Rwstat{}, nil
	case sessp.TTwatch:
		return &sp.Twatch{}, nil
	case sessp.TTreadF:
		return &sp.TreadF{}, nil
	case sessp.TTwriteF:
		return &sp.TwriteF{}, nil
	case sessp.TTrenameat:
		return &sp.Trenameat{}, nil
	case sessp.TRrenameat:
		return &sp.Rrenameat{}, nil
	case sessp.TTremovefile:
		return &sp.Tremovefile{}, nil
	case sessp.TTgetfile:
		return &sp.Tgetfile{}, nil
	case sessp.TTputfile:
		return &sp.Tputfile{}, nil
	case sessp.TTdetach:
		return &sp.Tdetach{}, nil
	case sessp.TRdetach:
		return &sp.Rdetach{}, nil
	case sessp.TTheartbeat:
		return &sp.Theartbeat{}, nil
	case sessp.TRheartbeat:
		return &sp.Rheartbeat{}, nil
	case sessp.TTwriteread:
		return &sp.Twriteread{}, nil
	}
	return nil, serr.NewErr(serr.TErrBadFcall, fmt.Sprintf("unknown type: %v", (uint64)(typ)))
}
