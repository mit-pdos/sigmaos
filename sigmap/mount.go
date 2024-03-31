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
	return fmt.Sprintf("{addr %v root %q}", mnt.Addr, mnt.Root)
}

func NullMount() *Tmount {
	return &Tmount{&TmountProto{}}
}

func NewMount(b []byte) (*Tmount, *serr.Err) {
	mnt := NullMount()
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
	mnt.Addr = addr
}

func (mnt *Tmount) Marshal() ([]byte, error) {
	return proto.Marshal(mnt)
}

func (mnt *Tmount) Address() *Taddr {
	return mnt.Addr[0]
}

func (mnt *Tmount) Addresses() Taddrs {
	return mnt.Addr
}

func NewMountService(srvaddrs Taddrs) *Tmount {
	return &Tmount{&TmountProto{Addr: srvaddrs}}
}

func NewMountServer(addr *Taddr) *Tmount {
	addrs := []*Taddr{addr}
	return NewMountService(addrs)
}

func (mnt *Tmount) TargetIPPort(idx int) (Tip, Tport) {
	return mnt.Addr[idx].GetIP(), mnt.Addr[idx].GetPort()
}
