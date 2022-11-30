package npcodec

import (
	"fmt"

	"sigmaos/fcall"
	np "sigmaos/sigmap"
)

// Adopted from https://github.com/docker/go-p9p/message.go

func newMsg(typ fcall.Tfcall) (np.Tmsg, *fcall.Err) {
	switch typ {
	case fcall.TTversion:
		return &np.Tversion{}, nil
	case fcall.TRversion:
		return &np.Rversion{}, nil
	case fcall.TTauth:
		return &np.Tauth{}, nil
	case fcall.TRauth:
		return &np.Rauth{}, nil
	case fcall.TTattach:
		return &np.Tattach{}, nil
	case fcall.TRattach:
		return &np.Rattach{}, nil
	case fcall.TRerror:
		return &np.Rerror{}, nil
	case fcall.TTflush:
		return &np.Tflush{}, nil
	case fcall.TRflush:
		return &np.Rflush{}, nil
	case fcall.TTwalk:
		return &np.Twalk{}, nil
	case fcall.TRwalk:
		return &np.Rwalk{}, nil
	case fcall.TTopen:
		return &np.Topen{}, nil
	case fcall.TRopen:
		return &np.Ropen{}, nil
	case fcall.TTcreate:
		return &np.Tcreate{}, nil
	case fcall.TRcreate:
		return &np.Rcreate{}, nil
	case fcall.TTread:
		return &np.Tread{}, nil
	case fcall.TRread:
		return &np.Rread{}, nil
	case fcall.TTwrite:
		return &np.Twrite{}, nil
	case fcall.TRwrite:
		return &np.Rwrite{}, nil
	case fcall.TTclunk:
		return &np.Tclunk{}, nil
	case fcall.TRclunk:
		return &np.Rclunk{}, nil // no response body
	case fcall.TTremove:
		return &np.Tremove{}, nil
	case fcall.TRremove:
		return &np.Rremove{}, nil
	case fcall.TTstat:
		return &np.Tstat{}, nil
	case fcall.TRstat:
		return &np.Rstat{}, nil
	case fcall.TTwstat:
		return &np.Twstat{}, nil
	case fcall.TRwstat:
		return &np.Rwstat{}, nil
	case fcall.TTwatch:
		return &np.Twatch{}, nil
	case fcall.TTreadV:
		return &np.TreadV{}, nil
	case fcall.TTwriteV:
		return &np.TwriteV{}, nil
	case fcall.TTrenameat:
		return &np.Trenameat{}, nil
	case fcall.TRrenameat:
		return &np.Rrenameat{}, nil
	case fcall.TTremovefile:
		return &np.Tremovefile{}, nil
	case fcall.TTgetfile:
		return &np.Tgetfile{}, nil
	case fcall.TRgetfile:
		return &np.Rgetfile{}, nil
	case fcall.TTsetfile:
		return &np.Tsetfile{}, nil
	case fcall.TTputfile:
		return &np.Tputfile{}, nil
	case fcall.TTdetach:
		return &np.Tdetach{}, nil
	case fcall.TRdetach:
		return &np.Rdetach{}, nil
	case fcall.TTheartbeat:
		return &np.Theartbeat{}, nil
	case fcall.TRheartbeat:
		return &np.Rheartbeat{}, nil
	case fcall.TTwriteread:
		return &np.Twriteread{}, nil
	case fcall.TRwriteread:
		return &np.Rwriteread{}, nil
	}
	return nil, fcall.MkErr(fcall.TErrBadFcall, fmt.Sprintf("unknown type: %v", (uint64)(typ)))
}
