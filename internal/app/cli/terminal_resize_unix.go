//go:build !windows

package cli

import (
	"os"
	"syscall"
)

func terminalResizeSignal() os.Signal {
	return syscall.SIGWINCH
}
