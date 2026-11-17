package msxls

import (
	"flag"
	"os"
	"testing"
)

// minimizeTime is the time that the fuzzer spends to make an input smaller
// when nobody sets -fuzzminimizetime.
//
// The fuzzer makes each input that finds new code smaller before it goes
// on. A step of that work removes each range of bytes in turn, so it runs
// the test about n*n/2 times for an input of n bytes. The workbooks of the
// seeds are Compound Files of some kilobytes, which the fuzzer cannot cut
// at their end, and a run of the test takes some milliseconds. Each new
// input can then take the whole default time of 60 seconds, as it does in
// the fuzz test of msppt.
const minimizeTime = "2s"

func TestMain(m *testing.M) {
	flag.Parse()
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "test.fuzzminimizetime" {
			set = true
		}
	})
	if !set {
		if err := flag.Set("test.fuzzminimizetime", minimizeTime); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}
