package sessdev

const (
	DATA  = "data-"
	CTL   = "ctl"
	CLONE = "clone-"
)

func CloneName(fn string) string {
	return CLONE + fn
}

func SidName(sid string, fn string) string {
	return sid + "-" + fn
}

func DataName(fn string) string {
	return DATA + fn
}
