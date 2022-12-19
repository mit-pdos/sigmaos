package sigmap

import (
	"strings"

	"google.golang.org/protobuf/proto"

	"sigmaos/fcall"
)

func NullMount() Tmount {
	return Tmount{}
}

func MkMount(b []byte) (Tmount, *fcall.Err) {
	mnt := NullMount()
	if err := proto.Unmarshal(b, &mnt); err != nil {
		return mnt, fcall.MkErrError(err)
	}
	return mnt, nil
}

func (mnt *Tmount) SetTree(tree string) {
	mnt.Root = tree
}

func (mnt Tmount) Marshal() ([]byte, error) {
	return proto.Marshal(&mnt)
}

func (mnt Tmount) Address() string {
	return mnt.Addr[0]
}

func MkMountService(srvaddrs []string) Tmount {
	return Tmount{Addr: srvaddrs}
}

func MkMountServer(addr string) Tmount {
	return MkMountService([]string{addr})
}

// IPv6: [::]:port
// IPv4: host:port
func (mnt Tmount) TargetIp() string {
	parts := strings.Split(mnt.Addr[0], ":")
	return parts[0]
}
