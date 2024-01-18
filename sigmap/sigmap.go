package sigmap

//
// Go structures for sigmap protocol, which is based on 9P.
//

func VEq(v1, v2 TQversion) bool {
	return v1 == NoV || v1 == v2
}
