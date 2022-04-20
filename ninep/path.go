package ninep

import (
	"regexp"
	"strings"
)

type Path []string

var slash *regexp.Regexp

func init() {
	slash = regexp.MustCompile(`//+`)
}

func Split(p string) Path {
	if p == "" {
		return Path{}
	}
	p = strings.TrimRight(p, "/")
	p = slash.ReplaceAllString(p, "/")
	path := strings.Split(p, "/")
	return path
}

func (path Path) String() string {
	s := strings.Join(path, "/")
	return s
}

func (path Path) Append(e string) Path {
	return append(path, e)
}

func (path Path) AppendPath(p Path) Path {
	return append(path, p...)
}

func (path Path) Copy() Path {
	p := make(Path, len(path))
	copy(p, path)
	return p
}

func EndSlash(p string) bool {
	return p[len(p)-1] == '/'
}

func (path1 Path) Eq(path2 Path) bool {
	if len(path1) != len(path2) {
		return false
	}
	for i := range path1 {
		if path1[i] != path2[i] {
			return false
		}
	}
	return true
}

// is c a child of p?
func (c Path) IsParent(parent Path) bool {
	if len(parent) == 0 { // p is root directory
		return true
	}
	for i := range parent {
		if i >= len(c) {
			return false
		}
		if parent[i] != c[i] {
			return false
		}
	}
	return true
}

func (path Path) Dir() Path {
	if len(path) < 1 {
		return Path{}
	}
	return path[0 : len(path)-1]
}

func (path Path) Base() string {
	if len(path) == 0 {
		return "."
	}
	return path[len(path)-1]
}

func IsUnionElem(elem string) bool {
	return strings.HasPrefix(elem, "~")
}

func (path Path) IsUnion() (Path, bool) {
	for i, c := range path {
		if IsUnionElem(c) {
			return path[:i], true
		}
	}
	return nil, false
}
