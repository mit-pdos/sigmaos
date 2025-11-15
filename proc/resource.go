package proc

type Tmcpu uint32 // If this type changes, make sure to change the typecasts below.
type Tmem uint32  // If this type changes, make sure to change the typecasts below.

func NewResourceReservation(mcpu Tmcpu, mem Tmem) *ResourceReservation {
	return &ResourceReservation{
		McpuInt: uint32(mcpu),
		MemInt:  uint32(mem),
	}
}
