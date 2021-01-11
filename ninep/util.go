package ninep

import (
	"strings"
)

func Split(path string) []string {
	p := strings.Split(path, "/")
	return p
}

func Join(path []string) string {
	p := strings.Join(path, "/")
	return p
}
