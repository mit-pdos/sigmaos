package sigmap

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

type Tendpoint struct {
	*TendpointProto
}

func NewEndpoint(t TTendpoint, srvaddrs Taddrs) *Tendpoint {
	return &Tendpoint{
		&TendpointProto{
			Claims: NewEndpointClaimsProto(t, srvaddrs),
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

// XXX Currently, endpoint type is a hint. In reality, it should be verified
// somehow by netproxy (e.g., by inspecting the IP addrs)
func NewEndpointClaimsProto(t TTendpoint, addrs Taddrs) *TendpointClaimsProto {
	return &TendpointClaimsProto{
		EndpointType: uint32(t),
		Addr:         addrs,
	}
}

func (ep *Tendpoint) SetType(t TTendpoint) {
	ep.Claims.EndpointType = uint32(t)
}

func (ep *Tendpoint) Type() TTendpoint {
	return TTendpoint(ep.Claims.EndpointType)
}

func (ep *Tendpoint) GetProto() *TendpointProto {
	return ep.TendpointProto
}

func (ep *Tendpoint) SetTree(tree string) {
	ep.Root = tree
}

func (ep *Tendpoint) SetAddr(addr Taddrs) {
	ep.Claims.Addr = addr
}

func (ep *Tendpoint) Marshal() ([]byte, error) {
	return proto.Marshal(ep)
}

func (ep *Tendpoint) Addrs() Taddrs {
	return ep.Claims.Addr
}

func (ep *Tendpoint) IsValidEP() bool {
	t := ep.Type()
	return t == EXTERNAL_EP || t == INTERNAL_EP
}

func (ep *Tendpoint) TargetIPPort(idx int) (Tip, Tport) {
	a := ep.Claims.Addr[idx]
	return a.GetIP(), a.GetPort()
}

func (ep *Tendpoint) String() string {
	if ep.TendpointProto == nil {
		return "<nil-endpoint-proto>"
	}
	return fmt.Sprintf("{ type:%v addr:%v root:%v }", ep.Type(), ep.Claims.Addr, ep.Root)
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
