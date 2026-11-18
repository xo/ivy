package interp

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"image"
	"image/jpeg"
	"strings"
	"testing"
	"time"

	"github.com/xo/ivy"
)

// run runs a program and returns the operand stack, from the bottom, as ==
// prints each object.
func run(t *testing.T, program string) (string, error) {
	t.Helper()
	in := New(Config{})
	in.Finish()
	err := in.Run(t.Context(), NewFile("test", []byte(program)))
	parts := make([]string, 0, len(in.os))
	for _, o := range in.os {
		parts = append(parts, in.repr(o, 0))
	}
	return strings.Join(parts, " "), err
}

func TestScanner(t *testing.T) {
	t.Parallel()
	tests := []struct {
		program, want string
	}{
		{"1 -2 +3 0", "1 -2 3 0"},
		{"1.5 -.002 34.5e1 1E-2 6.02E23", "1.5 -0.002 345.0 0.01 6.02e+23"},
		{"8#1777 16#FFFE 2#1000 36#Z", "1023 65534 8 35"},
		{"16#FFFFFFFF", "-1"},
		{"2147483648", "2.14748e+09"},
		{"/abc {abc_name}", "/abc {abc_name}"},
		{"(a(b)c) (\\n\\(\\)\\\\) (\\101\\0)", "(a\\(b\\)c) (\\12\\(\\)\\\\) (A\\0)"},
		{"(ab\\\ncd)", "(abcd)"},
		{"<41 42 4> <>", "(AB@) ()"},
		{"<~87cURD]j7BEbo7~>", "(Hello world)"},
		{"<~z~>", "(\\0\\0\\0\\0)"},
		{"{1 {2 add} exch} [ 1 2 ]", "{1 {2 add} exch} [1 2]"},
		{"% a comment\n1 %another\n2", "1 2"},
		{"/x 5 def { //x x }", "{5 x}"},
		{"<< /a 1 >> /a get", "1"},
		{"{1e 1.2.3 -}", "{1e 1.2.3 -}"},
	}
	for _, tt := range tests {
		got, err := run(t, tt.program)
		if err != nil {
			t.Errorf("%q: %v", tt.program, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%q = %q, want %q", tt.program, got, tt.want)
		}
	}
}

func TestBinaryTokens(t *testing.T) {
	t.Parallel()
	program := "\x84\x00\x00\x01\x00 \x88\xff \x8d\x01 \x8e\x02hi \x89\x28\x00\x80"
	got, err := run(t, program)
	if err != nil {
		t.Fatal(err)
	}
	if want := "256 -1 true (hi) 0.5"; got != want {
		t.Errorf("binary tokens = %q, want %q", got, want)
	}
}

// TestOperators runs examples of the operators, most from the PLRM.
func TestOperators(t *testing.T) {
	t.Parallel()
	tests := []struct {
		program, want string
	}{
		// stack
		{"1 2 3 pop", "1 2"},
		{"1 2 exch", "2 1"},
		{"1 2 3 2 copy", "1 2 3 2 3"},
		{"(a) (b) (c) 1 index", "(a) (b) (c) (b)"},
		{"(a) (b) (c) 3 -1 roll", "(b) (c) (a)"},
		{"(a) (b) (c) 3 1 roll", "(c) (a) (b)"},
		{"(a) (b) (c) 3 0 roll", "(a) (b) (c)"},
		{"1 mark 2 3 counttomark", "1 -mark- 2 3 2"},
		{"1 mark 2 3 cleartomark count", "1 1"},
		// arithmetic
		{"3 4 add 9.9 1.1 add", "7 11.0"},
		{"2147483647 1 add", "2.14748e+09"},
		{"3 2 div 4 2 div", "1.5 2.0"},
		{"3 2 idiv -5 2 idiv 4 2 idiv", "1 -2 2"},
		{"5 3 mod -5 3 mod", "2 -2"},
		{"-3 abs 4.5 neg", "3 -4.5"},
		{"3.2 ceiling -4.8 ceiling 99 ceiling", "4.0 -4.0 99"},
		{"3.2 floor -4.8 floor", "3.0 -5.0"},
		{"3.2 round 6.5 round -4.8 round -6.5 round", "3.0 7.0 -5.0 -6.0"},
		{"3.2 truncate -4.8 truncate", "3.0 -4.0"},
		{"16 sqrt", "4.0"},
		{"0 1 atan 1 0 atan -100 0 atan 4 4 atan", "0.0 90.0 270.0 45.0"},
		{"90 sin 0 cos", "1.0 1.0"},
		{"9 0.5 exp -9 -1 exp", "3.0 -0.111111"},
		{"100 log", "2.0"},
		// relational
		{"4.0 4 eq (abc) (abc) eq (abc) /abc eq [1 2] [1 2] eq", "true true true false"},
		{"4.2 4 gt (abc) (d) lt", "true true"},
		{"true false and 99 1 and 52 7 or true not 52 not", "false 1 55 false -53"},
		{"7 3 bitshift 142 -3 bitshift", "56 17"},
		// arrays and strings
		{"3 array length", "3"},
		{"[1 2 3] 1 get (abc) 0 get", "2 97"},
		{"[1 2 3] dup 1 (x) put", "[1 (x) 3]"},
		{"[9 8 7 6 5] 1 3 getinterval", "[8 7 6]"},
		{"[9 8 7 6 5] dup 1 [(a) (b)] putinterval", "[9 (a) (b) 6 5]"},
		{"[1 2] aload", "1 2 [1 2]"},
		{"(a) (b) 2 array astore", "[(a) (b)]"},
		{"0 [13 29 3] {add} forall", "45"},
		{"(abc) {} forall", "97 98 99"},
		{"(abbc) (ab) anchorsearch", "(bc) (ab) true"},
		{"(abbc) (bb) search", "(c) (bb) (a) true"},
		{"(abbc) (bd) search", "(abbc) false"},
		{"(15 (St1) {1 2 add}) token", "(\\(St1\\) {1 2 add}) 15 true"},
		{"() token", "false"},
		{"[1 2 3] [4 5 6 7] copy", "[1 2 3]"},
		{"1 2 3 3 packedarray dup length", "[1 2 3] 3"},
		// dictionaries
		{"/a 1 def a", "1"},
		{"5 dict dup /k (v) put /k get", "(v)"},
		{"<< /a 1 /b 2 >> length", "2"},
		{"/xyz where", "false"},
		{"/def where { pop true } if", "true"},
		{"/z 3 def /z 4 store z", "4"},
		{"<< /a 1 >> dup /a undef /a known", "false"},
		{"<< 1 (one) 1.0 (uno) >> 1 get", "(uno)"},
		{"<< (k) 1 >> /k get", "1"},
		{"countdictstack", "3"},
		{"<< /a 1 /b 2 >> {} forall", "/a 1 /b 2"},
		// control
		{"true {1} if false {2} if", "1"},
		{"false {1} {2} ifelse", "2"},
		{"0 1 1 4 {add} for", "10"},
		{"1 2 6 {} for", "1 3 5"},
		{"3 -.5 1 {} for", "3.0 2.5 2.0 1.5 1.0"},
		{"4 {(a)} repeat", "(a) (a) (a) (a)"},
		{"0 {1 add dup 5 eq {exit} if} loop", "5"},
		{"{1 2 add} exec", "3"},
		{"{1 stop 2} stopped", "1 true"},
		{"{1} stopped", "1 false"},
		{"{0 {1 add dup 3 eq {exit} if} loop} stopped", "3 false"},
		{"{ /nosuchname } stopped", "/nosuchname false"},
		{"{ nosuchname } stopped $error /errorname get", "true /undefined"},
		{"{ 1 0 idiv } stopped", "1 0 true"},
		{"{ (a) 1 add } stopped", "(a) 1 true"},
		// types
		{"1 type 1.0 type (a) type /a type [] type {} type << >> type", "integertype realtype stringtype nametype arraytype arraytype dicttype"},
		{"/a cvx xcheck {1} cvlit xcheck", "true false"},
		{"(3.3) cvi -47.8 cvi 520.9 cvi", "3 -47 520"},
		{"(3.3) cvr 1 cvr", "3.3 1.0"},
		{"(abc) cvn", "/abc"},
		{"123 10 string cvs /abc 10 string cvs true 10 string cvs 1.5 10 string cvs", "(123) (abc) (true) (1.5)"},
		{"123 16 10 string cvrs -1 16 10 string cvrs 123.4 10 10 string cvrs", "(7B) (FFFFFFFF) (123.4)"},
		{"[1 2] readonly wcheck (a) rcheck", "false true"},
		// conversion of names to procedures
		{"/f {2 mul} def 3 f", "6"},
		{"/f {dup 1 gt {dup 1 sub f mul} if} def 5 f", "120"},
		// save and restore
		{"/a [1 2] def save a 0 9 put restore a", "[1 2]"},
		{"/d 1 dict def save d /k 1 put restore d /k known", "false"},
		{"/x 1 def save /x 2 def restore x", "1"},
		// bind
		{"/f {add} bind def /add {sub} def 5 3 f", "8"},
		// files
		{"(abc) /ASCIIHexDecode filter pop", ""},
		{"(414243>) /ASCIIHexDecode filter 10 string readstring", "(ABC) false"},
		{"(87cURD]j7BEbo7~>) /ASCII85Decode filter 20 string readstring pop", "(Hello world)"},
		{"(\\002abc\\375x\\200) /RunLengthDecode filter 20 string readstring pop", "(abcxxxx)"},
		{"(abcEODdef) 0 (EOD) /SubFileDecode filter 20 string readstring pop", "(abc)"},
		{"(abcdef) 4 () /SubFileDecode filter 20 string readstring pop", "(abcd)"},
		{"{currentfile 3 string readstring pop} exec\nxyz", "(xyz)"},
		{"{currentfile 10 string readline pop} exec\nhello\n1", "(hello) 1"},
		{"{currentfile /ASCIIHexDecode filter 3 string readstring pop} exec\n41 42 43> 7", "(ABC) 7"},
		{"(a) (r) file", "/undefinedfilename"},
		// misc
		{"languagelevel", "3"},
		// the idioms of the procsets of Adobe Illustrator
		{"systemdict /languagelevel get 2 ge", "true"},
		{"currentsystemparams /MaxPatternCache get 0 gt", "true"},
		{"/Generic /Category findresource /Category get /Generic eq", "true"},
		{"/CSA /Generic /Category findresource dup length dict copy /Category defineresource pop " +
			"/x 5 /CSA defineresource pop /x /CSA findresource " +
			"/x /CSA /Category findresource begin ResourceStatus end", "5 1 0 true"},
		{"/y /CSA /Generic /Category findresource dup length dict copy /Category defineresource begin ResourceStatus end", "false"},
		{"{currentfile 0 (EOD) /SubFileDecode filter flushfile} exec\n<?xml version=\"1.0\"?> {[ EOD 7", "7"},
	}
	for _, tt := range tests {
		got, err := run(t, tt.program)
		if tt.want == "/undefinedfilename" {
			var je *JobError
			if !errors.As(err, &je) || je.Name != "undefinedfilename" {
				t.Errorf("%q: error %v, want undefinedfilename", tt.program, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", tt.program, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%q = %q, want %q", tt.program, got, tt.want)
		}
	}
}

func TestErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		program, name string
	}{
		{"nosuch", "undefined"},
		{"pop", "stackunderflow"},
		{"1 (a) add", "typecheck"},
		{"[1] 5 get", "rangecheck"},
		{"1 0 div", "undefinedresult"},
		{"end", "dictstackunderflow"},
		{"(abc", "syntaxerror"},
		{"exit", "invalidexit"},
		{"/a [1] readonly def a 0 2 put", "invalidaccess"},
	}
	for _, tt := range tests {
		_, err := run(t, tt.program)
		var je *JobError
		if !errors.As(err, &je) || je.Name != tt.name {
			t.Errorf("%q: error %v, want %s", tt.program, err, tt.name)
		}
	}
}

func TestErrorDict(t *testing.T) {
	t.Parallel()
	got, err := run(t, "errordict /undefined { pop (caught) } put nosuch 1")
	if err != nil {
		t.Fatal(err)
	}
	if want := "(caught) 1"; got != want {
		t.Errorf("handler = %q, want %q", got, want)
	}
}

// TestLimits makes sure that a program cannot run without end or take all
// the memory.
func TestLimits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		program string
		limit   bool
	}{
		{"{} loop", true},
		{"/f {f 1} def f", false},
		{"/f {1 f} def f", false},
		{"{1} loop", false},
		{"2147483647 string", false},
		{"0 1 100000 { pop 1000000 string pop } for", true},
		{"{ { } loop } stopped", true},
		{"10000000 array", false},
	}
	for _, tt := range tests {
		in := New(Config{Limits: ivy.Limits{MaxMemory: 64 << 20, MaxObjects: 1 << 16}})
		in.Finish()
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		err := in.Run(ctx, NewFile("test", []byte(tt.program)))
		cancel()
		if err == nil {
			t.Errorf("%q: no error", tt.program)
			continue
		}
		if tt.limit && !errors.Is(err, ivy.ErrLimit) {
			t.Errorf("%q: error %v, want ivy.ErrLimit", tt.program, err)
		}
	}
}

func TestCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	in := New(Config{})
	in.Finish()
	err := in.Run(ctx, NewFile("test", []byte("{} loop")))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error %v, want context.Canceled", err)
	}
}

func TestFlate(t *testing.T) {
	t.Parallel()
	// "hello hello hello" in zlib, as ASCIIHex
	program := "{currentfile /ASCIIHexDecode filter /FlateDecode filter 40 string readstring pop} exec\n" +
		"789ccb48cdc9c957c84090003a2e067d>\n7"
	got, err := run(t, program)
	if err != nil {
		t.Fatal(err)
	}
	if want := "(hello hello hello) 7"; got != want {
		t.Errorf("flate = %q, want %q", got, want)
	}
}

func TestDCT(t *testing.T) {
	t.Parallel()
	// a JPEG of 2 by 2 gray pixels, which the filter decodes to 4 bytes, and
	// no byte of the text after it
	img := image.NewGray(image.Rect(0, 0, 2, 2))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	program := "{currentfile /ASCIIHexDecode filter /DCTDecode filter 10 string readstring pop length} exec\n" +
		hex.EncodeToString(b.Bytes()) + ">\n7"
	got, err := run(t, program)
	if err != nil {
		t.Fatal(err)
	}
	if want := "4 7"; got != want {
		t.Errorf("dct = %q, want %q", got, want)
	}
}

func TestEexec(t *testing.T) {
	t.Parallel()
	// eexec runs the decrypted text with systemdict on the dictionary stack,
	// and the text after the closed file runs as usual
	plain := []byte("1234userdict /x 42 put currentfile closefile ")
	r := uint16(55665)
	var enc bytes.Buffer
	for _, p := range plain {
		c := p ^ byte(r>>8)
		r = (uint16(c)+r)*52845 + 22719
		enc.WriteByte(c)
	}
	program := "currentfile eexec\n" + hex.EncodeToString(enc.Bytes()) + "\n0000 cleartomark x"
	got, err := run(t, "mark "+program)
	if err != nil {
		t.Fatal(err)
	}
	if want := "42"; got != want {
		t.Errorf("eexec = %q, want %q", got, want)
	}
}

func FuzzScanner(f *testing.F) {
	for _, s := range []string{
		"1 2 add", "(a\\(b) <414> <~87cURD~>", "{ /a //b [1 2] << >> }", "16#FF 1.5e3",
		"\x84\x00\x00\x01\x00\x95\x20\x00\x02\x00\x01\x00\x02",
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(_ *testing.T, data []byte) {
		in := New(Config{Limits: ivy.Limits{MaxMemory: 16 << 20, MaxObjects: 1 << 14}})
		file := NewFile("fuzz", data)
		for range 10000 {
			_, ok, err := in.Token(file)
			if err != nil || !ok {
				return
			}
		}
	})
}
