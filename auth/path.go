package auth

import (
	"path/filepath"
)

// Test if the child path is in the directory subtree of the parent path
func IsInSubtree(cpath, ppath string) bool {
	// If the two paths are equal, cpath is trivially in ppath's subtree
	if ppath == cpath {
		return true
	}
	cp := filepath.SplitList(cpath)
	pp := filepath.SplitList(ppath)
	// Iterate through parent path
	for i := range pp {
		// If this element is a wildcard, then child path must be in the subtree
		if pp[i] == "*" {
			return true
		}
		// If the cpath has already ended, then it is not a subpath of the ppath
		// (it is a parent of the ppath)
		if i >= len(cp) {
			return false
		}
		// If the path elements don't match, the child isn't in the parent's
		// subtree
		if pp[i] != cp[i] {
			return false
		}
	}
	// All portions of the ppath match the cpath so far, and ppath is not equal
	// to cpath. Therefore cpath is not in ppath's subtree.
	return false
}
