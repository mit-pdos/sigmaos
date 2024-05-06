package sigmap

import (
	"fmt"
)

func (p Tperm) IsDir() bool        { return p&DMDIR == DMDIR }
func (p Tperm) IsSymlink() bool    { return p&DMSYMLINK == DMSYMLINK }
func (p Tperm) IsReplicated() bool { return p&DMREPL == DMREPL }
func (p Tperm) IsDevice() bool     { return p&DMDEVICE == DMDEVICE }
func (p Tperm) IsPipe() bool       { return p&DMNAMEDPIPE == DMNAMEDPIPE }
func (p Tperm) IsEphemeral() bool  { return p&DMTMP == DMTMP }
func (p Tperm) IsFile() bool       { return (p>>QTYPESHIFT)&0xFF == 0 }

func (p Tperm) String() string {
	qt := Qtype(p >> QTYPESHIFT)
	return fmt.Sprintf("{qt: %v qp: %x}", qt, uint8(p&TYPEMASK))
}
