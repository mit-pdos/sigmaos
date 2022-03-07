package repl

import (
	"encoding/json"
	"log"

	np "ulambda/ninep"
	"ulambda/npcodec"
)

func (rc *ReplyCache) Snapshot() []byte {
	entries := make(map[np.Tsession]map[np.Tseqno][]byte)
	for sess, m := range rc.entries {
		for seqno, rf := range m {
			if _, ok := entries[sess]; !ok {
				entries[sess] = make(map[np.Tseqno][]byte)
			}
			var b []byte
			var err1 *np.Err
			if rf.reply != nil {
				b, err1 = npcodec.Marshal(rf.reply)
				if err1 != nil {
					log.Fatalf("FATAL Error marshal np.Fcall in ReplyCache.Snapshot: %v, %v", err1, rf.reply)
				}
			}
			entries[sess][seqno] = b
		}
	}
	b, err := json.Marshal(entries)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding reply cache: %v", err)
	}
	return b
}

func Restore(b []byte) *ReplyCache {
	entries := make(map[np.Tsession]map[np.Tseqno][]byte)
	err := json.Unmarshal(b, &entries)
	if err != nil {
		log.Fatalf("FATAL error unmarshal ReplyCache in restore: %v", err)
	}
	rc := MakeReplyCache()
	for sess, m := range entries {
		for seqno, b := range m {
			if _, ok := rc.entries[sess]; !ok {
				rc.entries[sess] = make(map[np.Tseqno]*ReplyFuture)
			}

			fc := &np.Fcall{}
			if len(b) > 0 {
				err1 := npcodec.Unmarshal(b, fc)
				if err1 != nil {
					log.Fatalf("FATAL Error unmarshal np.Fcall in ReplyCache.Restore: %v, %v", err1, string(b))
				}
			}

			rf := MakeReplyFuture()
			if fc != nil {
				rf.Complete(fc)
			}
			rc.entries[sess][seqno] = rf
		}
	}
	return rc
}
