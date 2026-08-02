//go:build unix

package controller

import (
	"fmt"
	"os"
	"syscall"
)

func lockControllerConfig(path string) (func(), error) {
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // The path is the configured controller config.
	if err != nil {
		return nil, fmt.Errorf("open controller config lock: %w", err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("lock controller config: %w", err)
	}
	return func() {
		_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		_ = lock.Close()
	}, nil
}
