//go:build unix

package repo

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func configureGitCancellation(cmd *exec.Cmd) {
	// A canceled sync must also stop SSH/HTTP helpers, not just their Git parent.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
