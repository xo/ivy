package ivy_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// module is the path of the module.
const module = "github.com/xo/ivy"

// TestFormatsAreIndependent holds D6. The root package and the internal
// layers import no format, and a decoder imports no encoder, so that a
// program links only the formats that it imports.
func TestFormatsAreIndependent(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs go list")
	}
	pkgs := goList(t, "./...")
	for _, pkg := range pkgs {
		kind := kindOf(pkg)
		for _, dep := range goList(t, "-deps", pkg) {
			if dep == pkg || !strings.HasPrefix(dep, module+"/") {
				continue
			}
			switch depKind := kindOf(dep); {
			case kind == "root" && depKind != "internal":
				t.Errorf("%s imports %s. The root package imports no format. See D6", pkg, dep)
			case kind == "internal" && (depKind == "decoder" || depKind == "encoder"):
				t.Errorf("%s imports %s. An internal layer imports no format. See D6", pkg, dep)
			case kind == "decoder" && depKind == "encoder":
				t.Errorf("%s imports %s. A decoder imports no encoder. See D6", pkg, dep)
			}
		}
	}
}

// kindOf returns the kind of the package: root, internal, encoder or decoder.
func kindOf(pkg string) string {
	rel := strings.TrimPrefix(pkg, module)
	rel = strings.TrimPrefix(rel, "/")
	parts := strings.Split(rel, "/")
	switch {
	case rel == "":
		return "root"
	case parts[0] == "internal":
		return "internal"
	case strings.HasSuffix(parts[len(parts)-1], "enc"):
		return "encoder"
	}
	return "decoder"
}

// goList runs go list with args, and returns the import paths that it
// prints.
func goList(t *testing.T, args ...string) []string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "go", append([]string{"list"}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list %v: %v\n%s", args, err, stderr.String())
	}
	return strings.Fields(string(out))
}
