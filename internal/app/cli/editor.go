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
	runes := []rune(editor)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == '\\' && quote != '\'':
			if i+1 >= len(runes) {
				current.WriteRune('\\')
				continue
			}
			next := runes[i+1]
			if quote == '"' && next != '"' && next != '\\' && !isEditorCommandSpace(next) {
				current.WriteRune('\\')
				continue
			}
			current.WriteRune(next)
			i++
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case isEditorCommandSpace(r):
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
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

func isEditorCommandSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
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
