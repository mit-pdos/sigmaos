package clnt

type Tmethod string

const (
	START Tmethod = "Start"
	EVICT         = "Evict"
	EXIT          = "Exit"
)

func (m Tmethod) String() string {
	return string(m)
}

func (m Tmethod) Verb() string {
	switch m {
	case EVICT:
		return m.String()
	default:
		return m.String() + "ed"
	}
}
