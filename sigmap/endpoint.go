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
	mnt := NewNullEndpoint()
	if err := proto.Unmarshal(b, mnt); err != nil {
		return mnt, serr.NewErrError(err)
	}
	return mnt, nil
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

func (mnt *Tendpoint) IsSigned() bool {
	return mnt.Token != nil && mnt.Token.GetSignedToken() != NO_SIGNED_TOKEN
}

func (mnt *Tendpoint) GetProto() *TendpointProto {
	return mnt.TendpointProto
}

func (mnt *Tendpoint) SetToken(token *Ttoken) {
	mnt.Token = token
}

func (mnt *Tendpoint) SetTree(tree string) {
	mnt.Root = tree
}

func (mnt *Tendpoint) SetAddr(addr Taddrs) {
	mnt.Claims.Addr = addr
}

func (mnt *Tendpoint) Marshal() ([]byte, error) {
	return proto.Marshal(mnt)
}

func (mnt *Tendpoint) GetRealm() Trealm {
	return Trealm(mnt.Claims.GetRealmStr())
}

func (mnt *Tendpoint) Addrs() Taddrs {
	return mnt.Claims.Addr
}

func (mnt *Tendpoint) TargetIPPort(idx int) (Tip, Tport) {
	a := mnt.Claims.Addr[idx]
	return a.GetIP(), a.GetPort()
}

func (mnt *Tendpoint) String() string {
	return fmt.Sprintf("{ addr:%v realm:%v root:%v signed:%v }", mnt.Claims.Addr, Trealm(mnt.Claims.RealmStr), mnt.Root, mnt.IsSigned())
}
