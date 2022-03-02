package repl

import (
	"encoding/json"
	"log"

	np "ulambda/ninep"
)

func (rc *ReplyCache) Snapshot() []byte {
	entries := make(map[np.Tsession]map[np.Tseqno]np.Tmsg)
	for sess, m := range rc.entries {
		for seqno, rf := range m {
			if _, ok := entries[sess]; !ok {
				entries[sess] = make(map[np.Tseqno]np.Tmsg)
			}
			entries[sess][seqno] = rf.reply
		}
	}
	b, err := json.Marshal(entries)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding reply cache: %v", err)
	}
	return b
}

func Restore(b []byte) *ReplyCache {
	entries := make(map[np.Tsession]map[np.Tseqno]np.Tmsg)
	err := json.Unmarshal(b, entries)
	if err != nil {
		log.Fatalf("FATAL error unmarshal ReplyCache in restore: %v", err)
	}
	rc := MakeReplyCache()
	for sess, m := range entries {
		for seqno, msg := range m {
			if _, ok := entries[sess]; !ok {
				rc.entries[sess] = make(map[np.Tseqno]*ReplyFuture)
			}
			rf := MakeReplyFuture()
			if msg != nil {
				rf.Complete(msg)
			}
			rc.entries[sess][seqno] = rf
		}
	}
	return rc
}
