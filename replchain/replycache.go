package replchain

import (
	np "ulambda/ninep"
)

type Reply struct {
	fcall *np.Fcall
	frame []byte
}

type ReplyCache struct {
	entries map[np.Tsession]map[np.Tseqno]*Reply
}

func MakeReplyCache() *ReplyCache {
	rc := &ReplyCache{}
	rc.entries = map[np.Tsession]map[np.Tseqno]*Reply{}
	return rc
}

func (rc *ReplyCache) Put(request *np.Fcall, reply *np.Fcall, replyFrame []byte) {
	if _, ok := rc.entries[request.Session]; !ok {
		rc.entries[request.Session] = map[np.Tseqno]*Reply{}
	}
	rc.entries[request.Session][request.Seqno] = &Reply{reply, replyFrame}
}

// XXX Will need to handle entries which are "too old" eventually once we
// start evicting entries from the cache.
func (rc *ReplyCache) Get(request *np.Fcall) (*Reply, bool) {
	if sessionMap, ok := rc.entries[request.Session]; !ok {
		return nil, false
	} else {
		reply, ok := sessionMap[request.Seqno]
		return reply, ok
	}
}
