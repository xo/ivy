package review

import (
	"slices"
	"strings"
	"testing"
)

func TestReadManifest(t *testing.T) {
	const sum = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	in := "# the corpus\n" +
		"name\tkind\turl\tsha256\tsize\tlicense\tsource\tnote\n" +
		"\n" +
		"pdf/a.pdf\tPDF\thttps://example.com/a.pdf\t" + strings.ToUpper(sum) + "\t4\tMIT\texample\tpassword=XYZ; two columns\n" +
		"repo/sample.docx\tdocx\trepo:docx/testdata/sample.docx\n" +
		"b.txt\ttxt\thttps://example.com/b.txt\t\t\t\t\t\r\n"
	entries, err := ReadManifest(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got: %d", len(entries))
	}
	a := entries[0]
	if a.Name != "pdf/a.pdf" || a.Kind != "pdf" || a.SHA256 != sum || a.Size != 4 || a.License != "MIT" || a.Source != "example" || a.Line != 4 {
		t.Errorf("unexpected first entry: %+v", a)
	}
	if got := a.Password(); got != "XYZ" {
		t.Errorf("expected the password XYZ, got: %q", got)
	}
	b := entries[1]
	if p, ok := b.RepoPath(); !ok || p != "docx/testdata/sample.docx" {
		t.Errorf("expected the repository path docx/testdata/sample.docx, got: %q %v", p, ok)
	}
	if b.Size != -1 || b.Password() != "" {
		t.Errorf("expected no size and no password, got: %+v", b)
	}
	if got, want := b.Path("/repo", "/cache"), "/repo/docx/testdata/sample.docx"; got != want {
		t.Errorf("expected the path %s, got: %s", want, got)
	}
	if got, want := a.Path("/repo", "/cache"), "/cache/files/pdf/a.pdf"; got != want {
		t.Errorf("expected the path %s, got: %s", want, got)
	}
}

func TestReadManifestErrors(t *testing.T) {
	const header = "name\tkind\turl\tsha256\tsize\tlicense\tsource\tnote\n"
	for _, tc := range []struct {
		name, in, want string
	}{
		{"no header", "", "no header"},
		{"bad header", "name\tkind\turl\n", "expected the header"},
		{"short", header + "a.pdf\tpdf\n", "at least"},
		{"long", header + "a.pdf\tpdf\thttps://x/a\t\t\t\t\t\textra\n", "at most"},
		{"http", header + "a.pdf\tpdf\thttp://x/a\n", "https"},
		{"parent", header + "../a.pdf\tpdf\thttps://x/a\n", "clean relative path"},
		{"absolute", header + "/a.pdf\tpdf\thttps://x/a\n", "clean relative path"},
		{"repo parent", header + "a.pdf\tpdf\trepo:../a.pdf\n", "repository path"},
		{"sha", header + "a.pdf\tpdf\thttps://x/a\tabc\n", "sha256"},
		{"size", header + "a.pdf\tpdf\thttps://x/a\t\t-1\n", "size"},
		{"kind", header + "a.pdf\t\thttps://x/a\n", "kind"},
		{"duplicate", header + "a.pdf\tpdf\thttps://x/a\na.pdf\tpdf\thttps://x/b\n", "also on line 2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadManifest(strings.NewReader(tc.in))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected an error with %q, got: %v", tc.want, err)
			}
		})
	}
}

func TestSelectPages(t *testing.T) {
	for _, tc := range []struct {
		n      int
		counts []int
		want   []int
	}{
		{3, []int{10}, []int{0, 1, 2, 9}},
		{3, []int{2}, []int{0, 1}},
		{3, []int{10, 12}, []int{0, 1, 2, 9, 11}},
		{3, []int{-1, 5}, []int{0, 1, 2, 4}},
		{0, []int{4}, []int{0, 1, 2, 3}},
		{3, []int{-1, -1}, []int{}},
	} {
		got := selectPages(tc.n, tc.counts...)
		if !slices.Equal(got, tc.want) {
			t.Errorf("selectPages(%d, %v): expected %v, got: %v", tc.n, tc.counts, tc.want, got)
		}
	}
}
