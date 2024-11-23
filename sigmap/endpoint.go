package sigmap

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

type Tendpoint struct {
	*TendpointProto
}

// XXX Currently, endpoint type is a hint. In reality, it should be verified
// by dialproxy (e.g., by inspecting the IP addrs)
func NewEndpoint(t TTendpoint, srvaddrs Taddrs) *Tendpoint {
	return &Tendpoint{
		&TendpointProto{
			Type: uint32(t),
			Addr: srvaddrs,
		},
	}
}

func NewEndpointFromBytes(b []byte) (*Tendpoint, error) {
	ep := NewEndpoint(0, nil)
	if err := proto.Unmarshal(b, ep); err != nil {
		return ep, err
	}
	return ep, nil
}

func NewEndpointFromProto(p *TendpointProto) *Tendpoint {
	return &Tendpoint{p}
}

func (ep *Tendpoint) SetType(t TTendpoint) {
	ep.Type = uint32(t)
}

func (ep *Tendpoint) GetType() TTendpoint {
	return TTendpoint(ep.Type)
}

func (ep *Tendpoint) GetProto() *TendpointProto {
	return ep.TendpointProto
}

func (ep *Tendpoint) SetTree(tree string) {
	ep.Root = tree
}

func (ep *Tendpoint) SetAddr(addr Taddrs) {
	ep.Addr = addr
}

func (ep *Tendpoint) Marshal() ([]byte, error) {
	return proto.Marshal(ep)
}

func (ep *Tendpoint) Addrs() Taddrs {
	return ep.Addr
}

func (ep *Tendpoint) TargetIPPort(idx int) (Tip, Tport) {
	a := ep.Addr[idx]
	return a.GetIP(), a.GetPort()
}

func (ep *Tendpoint) String() string {
	if ep.TendpointProto == nil {
		return "<nil-endpoint-proto>"
	}
	return fmt.Sprintf("{ type:%v addr:%v root:%v }", ep.GetType(), ep.Addr, ep.Root)
}

func (t TTendpoint) String() string {
	switch t {
	case EXTERNAL_EP:
		return "E"
	case INTERNAL_EP:
		return "I"
	default:
		return "UNKOWN"
	}
}
