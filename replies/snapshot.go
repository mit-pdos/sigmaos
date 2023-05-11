package replies

import (
	"bytes"
	"encoding/json"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
	"sigmaos/spcodec"
)

type ReplyTableSnapshot struct {
	Entries map[sessp.Tseqno][]byte
	Sid     sessp.Tsession
}

func MakeReplyTableSnapshot() *ReplyTableSnapshot {
	return &ReplyTableSnapshot{
		Entries: make(map[sessp.Tseqno][]byte),
	}
}

func (rt *ReplyTable) Snapshot() []byte {
	rts := MakeReplyTableSnapshot()
	rts.Sid = rt.sid
	for seqno, rf := range rt.entries {
		var b []byte
		var err1 *serr.Err
		if rf.reply != nil {
			b, err1 = spcodec.MarshalFcallAndData(rf.reply)
			if err1 != nil {
				db.DFatalf("Error marshal sp.Fcall in ReplyTable.Snapshot: %v, %v", err1, rf.reply)
			}
		}
		rts.Entries[seqno] = b
	}
	b, err := json.Marshal(rts)
	if err != nil {
		db.DFatalf("Error snapshot encoding reply cache: %v", err)
	}
	return b
}

func Restore(b []byte) *ReplyTable {
	rts := MakeReplyTableSnapshot()
	err := json.Unmarshal(b, &rts)
	if err != nil {
		db.DFatalf("error unmarshal ReplyTable in restore: %v", err)
	}
	rt := MakeReplyTable(rts.Sid)
	for seqno, b := range rts.Entries {
		_, fm, err1 := spcodec.ReadUnmarshalFcallAndData(bytes.NewReader(b))
		if len(b) != 0 && err1 != nil {
			db.DFatalf("Error unmarshal sp.Fcall in ReplyTable.Restore: %v, %v", err1, string(b))
		}

		if fm != nil {
			rf := MakeReplyFuture()
			rf.Complete(fm)
			rt.entries[seqno] = rf
		}
	}
	return rt
}
