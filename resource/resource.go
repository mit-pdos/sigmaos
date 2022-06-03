package resource

import (
	"encoding/json"
	"fmt"

	db "ulambda/debug"
)

type ResourceGrantHandler func(*ResourceMsg)
type ResourceRequestHandler func(*ResourceMsg)

type Tresource uint32
type Tmsg uint32

const (
	Tnode Tresource = iota
	Trealm
)

func (rt Tresource) String() string {
	switch rt {
	case Tnode:
		return "Tnode"
	case Trealm:
		return "Trealm"
	default:
		db.DFatalf("Unknown resource type: %v", uint32(rt))
		return "Tunknown"
	}
}

const (
	Tgrant Tmsg = iota
	Trequest
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

type ResourceMsg struct {
	MsgType      Tmsg
	ResourceType Tresource
	Name         string
	Amount       int
}

func MakeResourceMsg(mt Tmsg, rt Tresource, n string, a int) *ResourceMsg {
	return &ResourceMsg{mt, rt, n, a}
}

func (r *ResourceMsg) Marshal() []byte {
	b, err := json.Marshal(r)
	if err != nil {
		db.DFatalf("Marshal: %v", err)
	}
	return b
}

func (r *ResourceMsg) Unmarshal(b []byte) {
	if err := json.Unmarshal(b, r); err != nil {
		db.DFatalf("Unmarshal: %v", err)
	}
}

func (r *ResourceMsg) String() string {
	return fmt.Sprintf("&{ MsgType: %v ResourceType:%v Name:%v Amount:%v }", r.MsgType, r.ResourceType, r.Name, r.Amount)
}
