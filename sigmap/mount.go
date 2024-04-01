package sigmap

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	"sigmaos/serr"
)

type Tmount struct {
	*TmountProto
}

func (mnt *Tmount) String() string {
	return fmt.Sprintf("{ addr:%v realm:%v root:%v }", mnt.Claims.Addr, Trealm(mnt.Claims.RealmStr), mnt.Root)
}

func NewMountClaimsProto(addrs Taddrs, realm Trealm) *TmountClaimsProto {
	return &TmountClaimsProto{
		RealmStr: realm.String(),
		Addr:     addrs,
	}
}

func NewNullMount() *Tmount {
	return &Tmount{
		&TmountProto{
			Claims: NewMountClaimsProto(nil, NOT_SET),
		},
	}
}

func NewMount(b []byte) (*Tmount, *serr.Err) {
	mnt := NewNullMount()
	if err := proto.Unmarshal(b, mnt); err != nil {
		return mnt, serr.NewErrError(err)
	}
	return mnt, nil
}

func NewMountFromProto(p *TmountProto) *Tmount {
	return &Tmount{p}
}

func (mnt *Tmount) GetProto() *TmountProto {
	return mnt.TmountProto
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

// TODO XXX kill
func (mnt *Tmount) Address() *Taddr {
	return mnt.Claims.Addr[0]
}

// XXX Dedup?
func (mnt *Tmount) Addresses() Taddrs {
	return mnt.Claims.Addr
}

// TODO XXX take in realm
func NewMountService(srvaddrs Taddrs) *Tmount {
	return &Tmount{
		&TmountProto{
			Claims: NewMountClaimsProto(srvaddrs, NOT_SET),
		},
	}
}

func NewMountServer(addr *Taddr) *Tmount {
	addrs := []*Taddr{addr}
	return NewMountService(addrs)
}

func (mnt *Tmount) TargetIPPort(idx int) (Tip, Tport) {
	a := mnt.Claims.Addr[idx]
	return a.GetIP(), a.GetPort()
}
