package fid

const (
	NullId uint64 = 0
	RootId uint64 = 1
)

// XXX maybe return version only on open?
type Fid struct {
	Version uint64
	Id      uint64
}

func MakeFid(v uint64, id uint64) Fid {
	return Fid{v, id}
}

func NullFid() Fid {
	return MakeFid(0, NullId)
}

func RootFid() Fid {
	return MakeFid(0, RootId)
}

type Ufid struct {
	Addr string
	Fid  Fid
}

func MakeUfid(addr string, fid Fid) *Ufid {
	return &Ufid{addr, fid}
}
