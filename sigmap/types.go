package sigmap

import (
	"fmt"
)

type Tfid uint32
type Tpath uint64
type Tdev uint32
type Tiounit uint32
type Tperm uint32
type Toffset uint64
type Tsize uint32
type Tlength uint64
type Tgid uint32
type Trealm string
type Tpid string
type TclntId uint64
type TleaseId uint64
type Tttl uint64
type Tip string
type Tport uint32

type Qtype uint32
type TQversion uint32

type Tmode uint32

type Taddrs []*Taddr

type TprincipalID string
type Tplatform string

type TTendpoint uint32

// XXX make its own type?
type Tsigmapath = string

type Tuid struct {
	Dev  Tdev
	Path Tpath
}

func (uid Tuid) String() string {
	return fmt.Sprintf("(%v,%d)", uid.Path, uid.Dev)
}
