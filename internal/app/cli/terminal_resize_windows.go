//go:build windows

package cli

import "os"

func terminalResizeSignal() os.Signal {
	return nil
}
