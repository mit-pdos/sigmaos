package netsrv

import (
	"sync"

	"ulambda/fid"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procinit"
	"ulambda/protclnt"
)

type ReplState struct {
	mu         *sync.Mutex
	HeadChan   *RelayNetConn
	TailChan   *RelayNetConn
	PrevChan   *RelayNetConn
	NextChan   *RelayNetConn
	ops        chan *RelayOp
	inFlight   *RelayOpSet
	fids       map[np.Tfid]*fid.Fid
	replyCache *ReplyCache
	*fslib.FsLib
	proc.ProcClnt
	*protclnt.Clnt
}

func MakeReplState() *ReplState {
	fsl := fslib.MakeFsLib("replstate")
	ops := make(chan *RelayOp)
	return &ReplState{&sync.Mutex{},
		nil, nil, nil, nil,
		ops,
		MakeRelayOpSet(),
		map[np.Tfid]*fid.Fid{},
		MakeReplyCache(),
		fsl,
		procinit.MakeProcClnt(fsl, procinit.GetProcLayersMap()),
		protclnt.MakeClnt(),
	}
}
