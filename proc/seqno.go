package proc

func NewProcSeqno(procqID, scheddID string, epoch, seqno uint64) *ProcSeqno {
	return &ProcSeqno{
		ProcqID:  procqID,
		ScheddID: scheddID,
		Epoch:    epoch,
		Seqno:    seqno,
	}
}
