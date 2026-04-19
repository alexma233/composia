package controller

import (
	"fmt"
	"os"
	"path/filepath"
)

func appendTaskLogRaw(logPath, content string) error {
	if logPath == "" || content == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return fmt.Errorf("create task log directory: %w", err)
	}
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open task log %q: %w", logPath, err)
	}

	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		return fmt.Errorf("write task log %q: %w", logPath, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close task log %q: %w", logPath, err)
	}
	return nil
}
