package fsobjsrv

import (
	np "ulambda/ninep"
)

type TQversionVec []np.TQversion

func NoVvec() TQversionVec   { return nil }
func MakeVvec() TQversionVec { return []np.TQversion{} }
func VvecEq(v1, v2 TQversionVec) bool {
	if v1 == nil {
		return true
	}
	if len(v1) != len(v2) {
		return false
	}
	for i, v := range v1 {
		if v != v2[i] {
			return false
		}
	}
	return true
}
