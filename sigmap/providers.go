package sigmap

import "log"

// Providers
type Tprovider uint32 // If this type changes, make sure to change the typecasts below.

const (
	T_ANY Tprovider = iota
	T_AWS
	T_CLOUDLAB
	t_valueMax // not exported
)

const (
	DEFAULT_PRVDR Tprovider = T_AWS
)

func (t Tprovider) String() string {
	switch t {
	case T_ANY:
		return "any"
	case T_AWS:
		return "aws"
	case T_CLOUDLAB:
		return "cloudlab"
	default:
		log.Fatalf("Unknown provider: %v", uint32(t))
	}
	return ""
}

func ParseTprovider(tstr string) Tprovider {
	switch tstr {
	case "any":
		return T_ANY
	case "aws":
		return T_AWS
	case "cloudlab":
		return T_CLOUDLAB
	default:
		log.Fatalf("Unknown provider: %v", tstr)
	}
	return 0
}

func (t Tprovider) TproviderToDir() string {
	return t.String() + "/"
}

func AllProviders() []Tprovider {
	var res []Tprovider
	for i := 1; i < int(t_valueMax); i++ {
		res = append(res, Tprovider(i))
	}
	return res
}
