package sigmap

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	"sigmaos/serr"
)

type Tendpoint struct {
	*TendpointProto
}

func NewEndpoint(srvaddrs Taddrs, realm Trealm) *Tendpoint {
	return &Tendpoint{
		&TendpointProto{
			Claims: NewEndpointClaimsProto(srvaddrs, realm),
			Token:  NoToken(),
		},
	}
}

func NewNullEndpoint() *Tendpoint {
	return NewEndpoint(nil, NOT_SET)
}

func NewEndpointFromBytes(b []byte) (*Tendpoint, *serr.Err) {
	ep := NewNullEndpoint()
	if err := proto.Unmarshal(b, ep); err != nil {
		return ep, serr.NewErrError(err)
	}
	return ep, nil
}

func NewEndpointFromProto(p *TendpointProto) *Tendpoint {
	return &Tendpoint{p}
}

func NewEndpointClaimsProto(addrs Taddrs, realm Trealm) *TendpointClaimsProto {
	return &TendpointClaimsProto{
		RealmStr: realm.String(),
		Addr:     addrs,
	}
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
	return fmt.Sprintf("{ addr:%v realm:%v root:%v signed:%v }", ep.Claims.Addr, Trealm(ep.Claims.RealmStr), ep.Root, ep.IsSigned())
}
