//go:build !unix

package review

import "os/exec"

// killGroup does nothing on a system without process groups. The end of
// the context kills only the first process.
func killGroup(*exec.Cmd) {}

// killRest does nothing on a system without process groups.
func killRest(*exec.Cmd) {}
