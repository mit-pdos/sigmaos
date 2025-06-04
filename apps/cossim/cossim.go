package cossim

import (
	"sigmaos/apps/cossim/proto"
	"sigmaos/util/rand"
)

const (
	COSSIMREL = "cossim"
	COSSIM    = "name/" + COSSIMREL
)

// Generate vectors filled with random numbers
func NewVectors(nvec int, dim int) []*proto.Vector {
	vecs := make([]*proto.Vector, nvec)
	for i := range vecs {
		vec := make([]float64, dim)
		for j := range vec {
			vec[j] = float64(rand.Uint64()) / float64(rand.Uint64()+1.0)
		}
		vecs[i] = &proto.Vector{
			Vals: vec,
		}
	}
	return vecs
}

func VectorToSlice(vec *proto.Vector) []float64 {
	// Construct input vec
	v := make([]float64, len(vec.Vals))
	for i := range v {
		v[i] = vec.Vals[i]
	}
	return v
}
