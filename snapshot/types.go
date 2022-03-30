package snapshot

type Tsnapshot uint8

const (
	Toverlay Tsnapshot = iota + 100
	Tdir
	Tfile
	Tsymlink
	Tfence
	Tstats
	Tsnapshotdev
)

func (s Tsnapshot) String() string {
	switch s {
	case Toverlay:
		return "Tsnapshot.Toverlay"
	case Tdir:
		return "Tsnapshot.Tdir"
	case Tfile:
		return "Tsnapshot.Tfile"
	case Tsymlink:
		return "Tsnapshot.Tsymlink"
	case Tfence:
		return "Tsnapshot.Tfence"
	case Tstats:
		return "Tsnapshot.Tstats"
	case Tsnapshotdev:
		return "Tsnapshot.Tsnapshotdev"
	default:
		return "Tsnapshot.Unknown"
	}
}
