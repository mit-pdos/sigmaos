package proc

import (
	"fmt"
)

type Tmcpu uint32 // If this type changes, make sure to change the typecasts below.
type Tmem uint32  // If this type changes, make sure to change the typecasts below.

type ResourceReservation struct {
	*ResourceReservationProto
}

func NewResourceReservation(mcpu Tmcpu, mem Tmem) *ResourceReservation {
	return &ResourceReservation{
		NewResourceReservationProto(mcpu, mem),
	}
}

func NewResourceReservationProto(mcpu Tmcpu, mem Tmem) *ResourceReservationProto {
	return &ResourceReservationProto{
		McpuInt: uint32(mcpu),
		MemInt:  uint32(mem),
	}
}

func (r *ResourceReservationProto) GetMcpu() Tmcpu {
	return Tmcpu(r.McpuInt)
}

func (r *ResourceReservationProto) SetMcpu(mcpu Tmcpu) {
	r.McpuInt = uint32(mcpu)
}

func (r *ResourceReservationProto) GetMem() Tmem {
	return Tmem(r.MemInt)
}

func (r *ResourceReservationProto) SetMem(mem Tmem) {
	r.MemInt = uint32(mem)
}

func (r *ResourceReservation) String() string {
	return fmt.Sprintf("&{ Mcpu:%v Mem:%v }", r.McpuInt, r.MemInt)
}
