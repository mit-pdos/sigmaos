package snapshot

type Tsnapshot uint8

const (
	Tdir Tsnapshot = iota
	Tfile
	Tsymlink
	Tstats
	Tsnapshotdev
)
