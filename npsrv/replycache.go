package npsrv

import (
	np "ulambda/ninep"
)

type Reply struct {
	fcall *np.Fcall
}

type ReplyCache struct {
	entries map[np.Tsession]map[np.Tseqno]*Reply
}

func MakeReplyCache() *ReplyCache {
	rc := &ReplyCache{}
	rc.entries = map[np.Tsession]map[np.Tseqno]*Reply{}
	return rc
}

func (rc *ReplyCache) Put(fcall *np.Fcall) {
	if _, ok := rc.entries[fcall.Session]; !ok {
		rc.entries[fcall.Session] = map[np.Tseqno]*Reply{}
	}
	rc.entries[fcall.Session][fcall.Seqno] = &Reply{fcall}
}

// XXX Will need to handle entries which are "too old" eventually once we
// start evicting entries from the cache.
func (rc *ReplyCache) Get(fcall *np.Fcall) (*Reply, bool) {
	if sessionMap, ok := rc.entries[fcall.Session]; !ok {
		return nil, false
	} else {
		reply, ok := sessionMap[fcall.Seqno]
		return reply, ok
	}
}
