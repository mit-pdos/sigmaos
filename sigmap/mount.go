package sigmap

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

type Tmount struct {
	*TmountProto
}

func NewMount(srvaddrs Taddrs, realm Trealm) *Tmount {
	return &Tmount{
		&TmountProto{
			Claims: NewMountClaimsProto(srvaddrs, realm),
			Token:  NoToken(),
		},
	}
}

func NewNullMount() *Tmount {
	return NewMount(nil, NOT_SET)
}

func NewMountFromBytes(b []byte) (*Tmount, error) {
	mnt := NewNullMount()
	if err := proto.Unmarshal(b, mnt); err != nil {
		return mnt, err
	}
	return mnt, nil
}

func NewMountFromProto(p *TmountProto) *Tmount {
	return &Tmount{p}
}

func NewMountClaimsProto(addrs Taddrs, realm Trealm) *TmountClaimsProto {
	return &TmountClaimsProto{
		RealmStr: realm.String(),
		Addr:     addrs,
	}
}

func (mnt *Tmount) IsSigned() bool {
	return mnt.Token != nil && mnt.Token.GetSignedToken() != NO_SIGNED_TOKEN
}

func (mnt *Tmount) GetProto() *TmountProto {
	return mnt.TmountProto
}

func (mnt *Tmount) SetToken(token *Ttoken) {
	mnt.Token = token
}

func (mnt *Tmount) SetTree(tree string) {
	mnt.Root = tree
}

func (mnt *Tmount) SetAddr(addr Taddrs) {
	mnt.Claims.Addr = addr
}

func (mnt *Tmount) Marshal() ([]byte, error) {
	return proto.Marshal(mnt)
}

func (mnt *Tmount) GetRealm() Trealm {
	return Trealm(mnt.Claims.GetRealmStr())
}

func (mnt *Tmount) Addrs() Taddrs {
	return mnt.Claims.Addr
}

func (mnt *Tmount) TargetIPPort(idx int) (Tip, Tport) {
	a := mnt.Claims.Addr[idx]
	return a.GetIP(), a.GetPort()
}

func (mnt *Tmount) String() string {
	return fmt.Sprintf("{ addr:%v realm:%v root:%v signed:%v }", mnt.Claims.Addr, Trealm(mnt.Claims.RealmStr), mnt.Root, mnt.IsSigned())
}
