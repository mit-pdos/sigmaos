package replies

import (
	"encoding/json"

	db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/npcodec"
)

func (rt *ReplyTable) Snapshot() []byte {
	entries := make(map[np.Tseqno][]byte)
	for seqno, rf := range rt.entries {
		var b []byte
		var err1 *np.Err
		if rf.reply != nil {
			b, err1 = npcodec.MarshalFcallMsgByte(rf.reply)
			if err1 != nil {
				db.DFatalf("Error marshal np.Fcall in ReplyTable.Snapshot: %v, %v", err1, rf.reply)
			}
		}
		entries[seqno] = b
	}
	b, err := json.Marshal(entries)
	if err != nil {
		db.DFatalf("Error snapshot encoding reply cache: %v", err)
	}
	return b
}

func Restore(b []byte) *ReplyTable {
	entries := make(map[np.Tseqno][]byte)
	err := json.Unmarshal(b, &entries)
	if err != nil {
		db.DFatalf("error unmarshal ReplyTable in restore: %v", err)
	}
	rt := MakeReplyTable()
	for seqno, b := range entries {
		fm, err1 := npcodec.UnmarshalFcallMsg(b)
		if len(b) != 0 && err1 != nil {
			db.DFatalf("Error unmarshal np.Fcall in ReplyTable.Restore: %v, %v", err1, string(b))
		}

		if fm != nil {
			rf := MakeReplyFuture()
			rf.Complete(fm)
			rt.entries[seqno] = rf
		}
	}
	return rt
}
