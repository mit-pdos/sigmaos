package repl

import (
	np "ulambda/ninep"
)

type ReplyCache struct {
	entries map[np.Tsession]map[np.Tseqno]np.Tmsg
}

func MakeReplyCache() *ReplyCache {
	rc := &ReplyCache{}
	rc.entries = map[np.Tsession]map[np.Tseqno]np.Tmsg{}
	return rc
}

func (rc *ReplyCache) Put(request *np.Fcall, reply np.Tmsg) {
	if _, ok := rc.entries[request.Session]; !ok {
		rc.entries[request.Session] = map[np.Tseqno]np.Tmsg{}
	}
	rc.entries[request.Session][request.Seqno] = reply
}

// XXX Will need to handle entries which are "too old" eventually once we
// start evicting entries from the cache.
func (rc *ReplyCache) Get(request *np.Fcall) (np.Tmsg, bool) {
	if sessionMap, ok := rc.entries[request.Session]; !ok {
		return nil, false
	} else {
		reply, ok := sessionMap[request.Seqno]
		return reply, ok
	}
}
