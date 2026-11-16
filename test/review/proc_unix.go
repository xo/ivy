//go:build unix

package review

import (
	"os/exec"
	"syscall"
)

// killGroup makes cmd start a process group of its own, and makes the end
// of its context kill the whole group. soffice and calibre start children
// that keep running when only the first process is killed.
func killGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
