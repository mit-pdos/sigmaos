package spcodec

import (
	"fmt"

	"sigmaos/fcall"
	sp "sigmaos/sigmap"
)

// Adopted from https://github.com/docker/go-p9p/message.go

func newMsg(typ fcall.Tfcall) (sp.Tmsg, *fcall.Err) {
	switch typ {
	case fcall.TTversion:
		return &sp.Tversion{}, nil
	case fcall.TRversion:
		return &sp.Rversion{}, nil
	case fcall.TTauth:
		return &sp.Tauth{}, nil
	case fcall.TRauth:
		return &sp.Rauth{}, nil
	case fcall.TTattach:
		return &sp.Tattach{}, nil
	case fcall.TRattach:
		return &sp.Rattach{}, nil
	case fcall.TRerror:
		return &sp.Rerror{}, nil
	case fcall.TTflush:
		return &sp.Tflush{}, nil
	case fcall.TRflush:
		return &sp.Rflush{}, nil
	case fcall.TTwalk:
		return &sp.Twalk{}, nil
	case fcall.TRwalk:
		return &sp.Rwalk{}, nil
	case fcall.TTopen:
		return &sp.Topen{}, nil
	case fcall.TRopen:
		return &sp.Ropen{}, nil
	case fcall.TTcreate:
		return &sp.Tcreate{}, nil
	case fcall.TRcreate:
		return &sp.Rcreate{}, nil
	case fcall.TTread:
		return &sp.Tread{}, nil
	case fcall.TRread:
		return &sp.Rread{}, nil
	case fcall.TTwrite:
		return &sp.Twrite{}, nil
	case fcall.TRwrite:
		return &sp.Rwrite{}, nil
	case fcall.TTclunk:
		return &sp.Tclunk{}, nil
	case fcall.TRclunk:
		return &sp.Rclunk{}, nil // no response body
	case fcall.TTremove:
		return &sp.Tremove{}, nil
	case fcall.TRremove:
		return &sp.Rremove{}, nil
	case fcall.TTstat:
		return &sp.Tstat{}, nil
	case fcall.TRstat:
		return &sp.Rstat{}, nil
	case fcall.TTwstat:
		return &sp.Twstat{}, nil
	case fcall.TRwstat:
		return &sp.Rwstat{}, nil
	case fcall.TTwatch:
		return &sp.Twatch{}, nil
	case fcall.TTreadV:
		return &sp.TreadV{}, nil
	case fcall.TTwriteV:
		return &sp.TwriteV{}, nil
	case fcall.TTrenameat:
		return &sp.Trenameat{}, nil
	case fcall.TRrenameat:
		return &sp.Rrenameat{}, nil
	case fcall.TTremovefile:
		return &sp.Tremovefile{}, nil
	case fcall.TTgetfile:
		return &sp.Tgetfile{}, nil
	case fcall.TRgetfile:
		return &sp.Rgetfile{}, nil
	case fcall.TTsetfile:
		return &sp.Tsetfile{}, nil
	case fcall.TTputfile:
		return &sp.Tputfile{}, nil
	case fcall.TTdetach:
		return &sp.Tdetach{}, nil
	case fcall.TRdetach:
		return &sp.Rdetach{}, nil
	case fcall.TTheartbeat:
		return &sp.Theartbeat{}, nil
	case fcall.TRheartbeat:
		return &sp.Rheartbeat{}, nil
	case fcall.TTwriteread:
		return &sp.Twriteread{}, nil
	case fcall.TRwriteread:
		return &sp.Rwriteread{}, nil
	}
	return nil, fcall.MkErr(fcall.TErrBadFcall, fmt.Sprintf("unknown type: %v", (uint64)(typ)))
}
