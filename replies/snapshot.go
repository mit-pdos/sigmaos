package replies

import (
	"encoding/json"

	db "sigmaos/debug"
	"sigmaos/fcall"
	sp "sigmaos/sigmap"
	"sigmaos/spcodec"
)

func (rt *ReplyTable) Snapshot() []byte {
	entries := make(map[sp.Tseqno][]byte)
	for seqno, rf := range rt.entries {
		var b []byte
		var err1 *fcall.Err
		if rf.reply != nil {
			b, err1 = spcodec.MarshalFcallMsgByte(rf.reply)
			if err1 != nil {
				db.DFatalf("Error marshal sp.Fcall in ReplyTable.Snapshot: %v, %v", err1, rf.reply)
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
	entries := make(map[sp.Tseqno][]byte)
	err := json.Unmarshal(b, &entries)
	if err != nil {
		db.DFatalf("error unmarshal ReplyTable in restore: %v", err)
	}
	rt := MakeReplyTable()
	for seqno, b := range entries {
		fm, err1 := spcodec.UnmarshalFcallMsg(b)
		if len(b) != 0 && err1 != nil {
			db.DFatalf("Error unmarshal sp.Fcall in ReplyTable.Restore: %v, %v", err1, string(b))
		}

		if fm != nil {
			rf := MakeReplyFuture()
			rf.Complete(fm.(*sp.FcallMsg))
			rt.entries[seqno] = rf
		}
	}
	return rt
}
