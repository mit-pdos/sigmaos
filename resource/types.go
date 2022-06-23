package resource

import (
	db "ulambda/debug"
)

type Tmsg uint32
type Tresource uint32

// Message types
const (
	Tgrant Tmsg = iota
	Trequest
)

// Resource types
const (
	Tnode Tresource = iota
	Trealm
	Tcore
)

func (mt Tmsg) String() string {
	switch mt {
	case Tgrant:
		return "Tgrant"
	case Trequest:
		return "Trequest"
	default:
		db.DFatalf("Unknown resource type: %v", uint32(mt))
		return "Tunknown"
	}
}

func (rt Tresource) String() string {
	switch rt {
	case Tnode:
		return "Tnode"
	case Trealm:
		return "Trealm"
	case Tcore:
		return "Tcore"
	default:
		db.DFatalf("Unknown resource type: %v", uint32(rt))
		return "Tunknown"
	}
}
