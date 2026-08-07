//go:build !windows

package agent

import (
	"errors"
	"io"
	"os/exec"
	"syscall"

	"github.com/creack/pty/v2"
)

const (
	composeTerminalCols = 160
	composeTerminalRows = 40
)

// runCommandWithPTYLiveLogs keeps terminal redraws intact instead of turning
// carriage returns into independent log lines.
func runCommandWithPTYLiveLogs(command *exec.Cmd, uploadLog func(string) error) error {
	prepareCommandForTerminalLog(command)
	command.Env = setCommandEnv(command.Env, "COLUMNS", "160")
	command.Env = setCommandEnv(command.Env, "LINES", "40")

	terminal, err := pty.StartWithSize(command, &pty.Winsize{
		Cols: composeTerminalCols,
		Rows: composeTerminalRows,
	})
	if err != nil {
		return err
	}

	writer := newCommandLogWriter(uploadLog, false)
	_, readErr := io.Copy(writer, terminal)
	_ = terminal.Close()

	if readErr != nil && !errors.Is(readErr, syscall.EIO) {
		_ = command.Process.Kill()
	}
	waitErr := command.Wait()
	if readErr != nil && !errors.Is(readErr, syscall.EIO) {
		return readErr
	}
	return waitErr
}
