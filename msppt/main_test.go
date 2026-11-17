package msppt

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
// the test about n*n/2 times for an input of n bytes. A presentation of
// the seeds has 43008 bytes, and a run of the test takes about 2 ms. Each
// new input then takes the whole default time of 60 seconds, and a run of
// 1 minute stops after about 17 inputs. ivy does not hang: each input runs
// in a few milliseconds.
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
