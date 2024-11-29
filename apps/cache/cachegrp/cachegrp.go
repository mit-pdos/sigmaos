package cachegrp

const (
	SRVDIR = "servers/"
)

func Server(n string) string {
	return SRVDIR + n
}
