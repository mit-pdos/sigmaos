package sigmap

import (
	"encoding/json"
	"log"
	"path/filepath"
	"strconv"
	"strings"
)

type Tepoch int64
type Tseqno uint64

const NoEpoch Tepoch = ^Tepoch(0)

func (e Tepoch) String() string {
	return strconv.FormatUint(uint64(e), 16)
}

func String2Epoch(epoch string) (Tepoch, error) {
	e, err := strconv.ParseUint(epoch, 16, 64)
	if err != nil {
		return Tepoch(0), err
	}
	return Tepoch(e), nil
}

type Tfence struct {
	PathName string
	Epoch    Tepoch
	Seqno    Tseqno
}

func NoFence() Tfence {
	return Tfence{}
}

func NullFence() *Tfence {
	return &Tfence{}
}

func NewFence(pn string, epoch Tepoch) Tfence {
	return Tfence{PathName: pn, Epoch: epoch}
}

func NewFenceJson(b []byte) (*Tfence, error) {
	f := NullFence()
	if err := json.Unmarshal(b, f); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *Tfence) Name() string {
	return strings.Replace(f.Prefix(), "/", "-", -1)
}

func (f *Tfence) HasFence() bool {
	return f.PathName != ""
}

func (f *Tfence) IsInitialized() bool {
	return f.Epoch > 0
}

func (f *Tfence) Prefix() string {
	return filepath.Dir(f.PathName)
}

func (f1 *Tfence) Upgrade(f2 *Tfence) {
	f1.Epoch = f2.Epoch
	f1.Seqno = f2.Seqno
}

func (f1 *Tfence) LessThan(f2 *Tfence) bool {
	return f1.Epoch < f2.Epoch ||
		(f1.Epoch == f2.Epoch && f1.Seqno < f2.Seqno)
}

func (f1 *Tfence) Eq(f2 *Tfence) bool {
	return f1.Epoch == f2.Epoch && f1.Seqno == f2.Seqno
}

type Tfencecmp = uint32

const (
	FENCE_EQ Tfencecmp = iota + 1
	FENCE_LT
	FENCE_GT
)

func (f1 *Tfence) Cmp(f2 *Tfence) Tfencecmp {
	if f1.Eq(f2) {
		return FENCE_EQ
	} else if f1.LessThan(f2) {
		return FENCE_LT
	} else {
		return FENCE_GT
	}
}

func (f *Tfence) FenceProto() *TfenceProto {
	fp := NewFenceProto()
	fp.PathName = f.PathName
	fp.Epoch = uint64(f.Epoch)
	fp.Seqno = uint64(f.Seqno)
	return fp
}

func (f Tfence) Json() []byte {
	b, err := json.Marshal(f)
	if err != nil {
		log.Printf("fence %v json marshal err %v\n", f, err)
		return nil
	}
	return b
}

func NewFenceProto() *TfenceProto {
	return &TfenceProto{}
}

func (fp *TfenceProto) HasFence() bool {
	return fp.PathName != ""
}

func (fp *TfenceProto) Tpathname() string {
	return fp.PathName
}

func (fp *TfenceProto) Tepoch() Tepoch {
	return Tepoch(fp.Epoch)
}

func (fp *TfenceProto) Tseqno() Tseqno {
	return Tseqno(fp.Seqno)
}

func (fp *TfenceProto) Tfence() Tfence {
	return Tfence{PathName: fp.PathName, Epoch: fp.Tepoch(), Seqno: fp.Tseqno()}
}
