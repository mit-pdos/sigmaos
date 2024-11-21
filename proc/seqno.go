package proc

func NewProcSeqno(procqID, mschedID string, epoch, seqno uint64) *ProcSeqno {
	return &ProcSeqno{
		ProcqID:  procqID,
		MSchedID: mschedID,
		Epoch:    epoch,
		Seqno:    seqno,
	}
}
