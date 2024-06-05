// Package path manipulates pathnames and provides Tpathname to represent
// pathnames as a slice of pathname components.
package path

import (
	"regexp"
	"strings"
)

type Tpathname []string

var slash *regexp.Regexp

func init() {
	slash = regexp.MustCompile(`//+`)
}

func Split(p string) Tpathname {
	if p == "" {
		return Tpathname{}
	}
	p = strings.TrimRight(p, "/")
	p = slash.ReplaceAllString(p, "/")
	path := strings.Split(p, "/")
	return path
}

func (path Tpathname) String() string {
	s := strings.Join(path, "/")
	return s
}

func (path Tpathname) Append(e string) Tpathname {
	return append(path, e)
}

func (path Tpathname) AppendPath(p Tpathname) Tpathname {
	return append(path, p...)
}

func (path Tpathname) Copy() Tpathname {
	p := make(Tpathname, len(path))
	copy(p, path)
	return p
}

func EndSlash(p string) bool {
	return p[len(p)-1] == '/'
}

func (path1 Tpathname) Equal(path2 Tpathname) bool {
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

// is c a child of parent?
func (c Tpathname) IsParent(parent Tpathname) bool {
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

func (path Tpathname) Dir() Tpathname {
	if len(path) < 1 {
		return Tpathname{}
	}
	return path[0 : len(path)-1]
}

func (path Tpathname) Base() string {
	if len(path) == 0 {
		return "."
	}
	return path[len(path)-1]
}

func IsUnionElem(elem string) bool {
	return strings.HasPrefix(elem, "~")
}

func (path Tpathname) IsUnion() (string, Tpathname, bool) {
	for i, c := range path {
		if IsUnionElem(c) {
			return path[:i].String(), path[i:], true
		}
	}
	return "", nil, false
}
