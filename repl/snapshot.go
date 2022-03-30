package repl

import (
	"encoding/json"

	db "ulambda/debug"
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
				b, err1 = npcodec.MarshalFcallByte(rf.reply)
				if err1 != nil {
					db.DFatalf("Error marshal np.Fcall in ReplyCache.Snapshot: %v, %v", err1, rf.reply)
				}
			}
			entries[sess][seqno] = b
		}
	}
	b, err := json.Marshal(entries)
	if err != nil {
		db.DFatalf("Error snapshot encoding reply cache: %v", err)
	}
	return b
}

func Restore(b []byte) *ReplyCache {
	entries := make(map[np.Tsession]map[np.Tseqno][]byte)
	err := json.Unmarshal(b, &entries)
	if err != nil {
		db.DFatalf("error unmarshal ReplyCache in restore: %v", err)
	}
	rc := MakeReplyCache()
	for sess, m := range entries {
		for seqno, b := range m {
			if _, ok := rc.entries[sess]; !ok {
				rc.entries[sess] = make(map[np.Tseqno]*ReplyFuture)
			}

			fc, err1 := npcodec.UnmarshalFcall(b)
			if len(b) != 0 && err1 != nil {
				db.DFatalf("Error unmarshal np.Fcall in ReplyCache.Restore: %v, %v", err1, string(b))
			}

			if fc != nil {
				rf := MakeReplyFuture()
				rf.Complete(fc)
				rc.entries[sess][seqno] = rf
			}
		}
	}
	return rc
}
