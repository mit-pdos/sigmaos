package fs

type Symlink struct {
	target string
}

func MakeSym(target string) *Symlink {
	s := Symlink{target}
	return &s
}
