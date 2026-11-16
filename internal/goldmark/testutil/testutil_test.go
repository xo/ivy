// Copied from github.com/yuin/goldmark, file testutil/testutil_test.go, at commit d473f9516b6b
// (D7, docs/UPSTREAM.md). Its MIT license is in internal/goldmark/LICENSE.

package testutil

import "testing"

// This will fail to compile if the TestingT interface is changed in a way
// that doesn't conform to testing.T.
var _ TestingT = (*testing.T)(nil)
