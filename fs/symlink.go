package fs

type Symlink struct {
	target string
}

func makeSym(target string) *Symlink {
	s := Symlink{target}
	return &s
}
