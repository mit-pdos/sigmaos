package sigmap

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

type Tendpoint struct {
	*TendpointProto
}

func NewEndpoint(t TTendpoint, srvaddrs Taddrs, realm Trealm) *Tendpoint {
	return &Tendpoint{
		&TendpointProto{
			Claims: NewEndpointClaimsProto(t, srvaddrs, realm),
			Token:  NoToken(),
		},
	}
}

func NewEndpointFromBytes(b []byte) (*Tendpoint, error) {
	ep := NewEndpoint(0, nil, NOT_SET)
	if err := proto.Unmarshal(b, ep); err != nil {
		return ep, err
	}
	return ep, nil
}

func NewEndpointFromProto(p *TendpointProto) *Tendpoint {
	return &Tendpoint{p}
}

// XXX Currently, endpitn type is a hint. In reality, it should be verified
// somehow by netproxy (e.g., by inspecting the IP addrs)
func NewEndpointClaimsProto(t TTendpoint, addrs Taddrs, realm Trealm) *TendpointClaimsProto {
	return &TendpointClaimsProto{
		RealmStr:     realm.String(),
		EndpointType: uint32(t),
		Addr:         addrs,
	}
}

func (ep *Tendpoint) Type() TTendpoint {
	return TTendpoint(ep.Claims.EndpointType)
}

func (ep *Tendpoint) IsSigned() bool {
	return ep.Token != nil && ep.Token.GetSignedToken() != NO_SIGNED_TOKEN
}

func (ep *Tendpoint) GetProto() *TendpointProto {
	return ep.TendpointProto
}

func (ep *Tendpoint) SetToken(token *Ttoken) {
	ep.Token = token
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

func (ep *Tendpoint) GetRealm() Trealm {
	return Trealm(ep.Claims.GetRealmStr())
}

func (ep *Tendpoint) Addrs() Taddrs {
	return ep.Claims.Addr
}

func (ep *Tendpoint) TargetIPPort(idx int) (Tip, Tport) {
	a := ep.Claims.Addr[idx]
	return a.GetIP(), a.GetPort()
}

func (ep *Tendpoint) String() string {
	return fmt.Sprintf("{ type:%v addr:%v realm:%v root:%v signed:%v }", ep.Type(), ep.Claims.Addr, Trealm(ep.Claims.RealmStr), ep.Root, ep.IsSigned())
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
