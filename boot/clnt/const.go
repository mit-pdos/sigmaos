package clnt

type Tboot string

const (
	BOOT_REALM   Tboot = "realm"
	BOOT_ALL           = "all"
	BOOT_NAMED         = "named"
	BOOT_NODE          = "node"
	BOOT_MINNODE       = "minnode"
)

func (b Tboot) String() string {
	return string(b)
}
