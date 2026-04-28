package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func editText(ctx context.Context, initialContent string, pattern string, mode os.FileMode) (string, bool, error) {
	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", false, fmt.Errorf("create editor temp file: %w", err)
	}
	path := file.Name()
	defer func() { _ = os.Remove(path) }()

	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return "", false, fmt.Errorf("set editor temp file mode: %w", err)
	}
	if _, err := file.WriteString(initialContent); err != nil {
		_ = file.Close()
		return "", false, fmt.Errorf("write editor temp file: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", false, fmt.Errorf("close editor temp file: %w", err)
	}

	if err := runEditor(ctx, path); err != nil {
		return "", false, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("read editor temp file: %w", err)
	}
	updated := string(content)
	return updated, updated != initialContent, nil
}

func runEditor(ctx context.Context, path string) error {
	editor := chooseEditor()
	cmd := exec.CommandContext(ctx, "sh", "-c", editor+" \"$1\"", "composia-editor", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run editor %q: %w", editor, err)
	}
	return nil
}

func chooseEditor() string {
	if editor := strings.TrimSpace(os.Getenv("VISUAL")); editor != "" {
		return editor
	}
	if editor := strings.TrimSpace(os.Getenv("EDITOR")); editor != "" {
		return editor
	}
	return "vi"
}
