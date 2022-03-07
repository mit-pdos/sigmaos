package snapshot

type Tsnapshot uint8

const (
	Tdir Tsnapshot = iota
	Tfile
	Tsymlink
	Tstats
	Tsnapshotdev
)

func (s Tsnapshot) String() string {
	switch s {
	case Tdir:
		return "Tsnapshot.Tdir"
	case Tfile:
		return "Tsnapshot.Tfile"
	case Tsymlink:
		return "Tsnapshot.Tsymlink"
	case Tstats:
		return "Tsnapshot.Tstats"
	case Tsnapshotdev:
		return "Tsnapshot.Tsnapshotdev"
	default:
		return "Tsnapshot.Unknown"
	}
}
