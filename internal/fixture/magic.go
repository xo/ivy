package fixture

import (
	"bytes"
	"testing"

	"github.com/xo/magic"
	"github.com/xo/magic/container"
)

// containerMIMEs are the MIME types that magic gives to a zip file, a
// Compound File or a Mobipocket book, whose format container can name.
var containerMIMEs = map[string]bool{
	"application/zip":                         true,
	"application/octet-stream":                true,
	"application/encrypted":                   true,
	"application/x-mobipocket-ebook":          true,
	"application/vnd.android.package-archive": true,
	"application/x-ios-app":                   true,
}

// DetectMIME returns the MIME type of buf as a program finds it, as
// magic D65 says. magic.Detect names the type of the file. When that type
// is a container, container.Detect names the format inside, and its MIME
// type replaces the one of magic. DetectMIME also returns the format of
// container, which is container.Unknown when magic names no container or
// when container names no format.
func DetectMIME(t *testing.T, buf []byte) (string, container.Format) {
	t.Helper()
	r, err := magic.Detect(t.Context(), buf)
	if err != nil {
		t.Fatalf("magic.Detect: %v", err)
	}
	if !containerMIMEs[r.MIME] {
		return r.MIME, container.Unknown
	}
	f, err := container.Detect(bytes.NewReader(buf), int64(len(buf)), container.Limits{})
	if err != nil {
		t.Fatalf("container.Detect: %v", err)
	}
	if f == container.Unknown {
		return r.MIME, f
	}
	return f.MIME(), f
}
