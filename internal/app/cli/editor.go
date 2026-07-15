package cli

import (
	"context"
	"errors"
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
	content, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return "", false, fmt.Errorf("read editor temp file: %w", err)
	}
	updated := string(content)
	return updated, updated != initialContent, nil
}

func runEditor(ctx context.Context, path string) error {
	editor := chooseEditor()
	parts, err := splitEditorCommand(editor)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, parts[0], append(parts[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run editor %q: %w", editor, err)
	}
	return nil
}

func splitEditorCommand(editor string) ([]string, error) {
	var parts []string
	var current strings.Builder
	var quote rune
	escaped := false
	for _, r := range editor {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("parse editor command %q: unterminated quote", editor)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	if len(parts) == 0 {
		return nil, errors.New("editor command is empty")
	}
	return parts, nil
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
