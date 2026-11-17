//go:build !unix

package review

import "testing"

// otherFileSystem returns an empty string: the tests do not look for
// another file system on a system that is not Unix.
func otherFileSystem(*testing.T, string) string {
	return ""
}
