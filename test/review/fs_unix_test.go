//go:build unix

package review

import (
	"os"
	"syscall"
	"testing"
)

// otherFileSystem returns a new folder on a file system other than the one
// of the folder than, or an empty string when the system has no such
// folder that a test can write.
func otherFileSystem(t *testing.T, than string) string {
	t.Helper()
	dev := func(path string) (uint64, bool) {
		var st syscall.Stat_t
		if err := syscall.Stat(path, &st); err != nil {
			return 0, false
		}
		return uint64(st.Dev), true //nolint:unconvert // the type of Dev differs between the systems
	}
	want, ok := dev(than)
	if !ok {
		t.Fatalf("reading the file system of %s", than)
	}
	for _, dir := range []string{"/dev/shm", "/var/tmp", os.TempDir()} {
		if d, ok := dev(dir); !ok || d == want {
			continue
		}
		tmp, err := os.MkdirTemp(dir, "ivy-review-test-") //nolint:usetesting // t.TempDir cannot pick the file system
		if err != nil {
			continue
		}
		t.Cleanup(func() { _ = os.RemoveAll(tmp) })
		return tmp
	}
	return ""
}
