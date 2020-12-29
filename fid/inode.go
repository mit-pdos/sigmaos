package fid

type IType int

const (
	FileT  IType = 1
	DirT   IType = 2
	DevT   IType = 3
	MountT IType = 4
	PipeT  IType = 5
	SymT   IType = 6
)
