package ninep

import (
	"strings"
)

func Split(path string) []string {
	path = strings.TrimRight(path, "/")
	p := strings.Split(path, "/")
	return p
}

func Join(path []string) string {
	p := strings.Join(path, "/")
	return p
}

func EndSlash(path string) bool {
	return path[len(path)-1] == '/'
}
