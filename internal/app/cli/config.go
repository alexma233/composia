package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	cliConfigKeyAddr      = "addr"
	cliConfigKeyTokenFile = "token_file"
)

type cliConfig map[string]string

func (application *app) runConfig(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia config <get|set|unset|path>")
	}
	switch args[0] {
	case "get":
		return application.runConfigGet(args[1:])
	case "set":
		return application.runConfigSet(args[1:])
	case "unset":
		return application.runConfigUnset(args[1:])
	case "path":
		return application.runConfigPath(args[1:])
	default:
		return fmt.Errorf("unknown config command %q", args[0])
	}
}

func (application *app) runConfigGet(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: composia config get [key]")
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}
	if len(args) == 1 {
		if !isCLIConfigKey(args[0]) {
			return fmt.Errorf("unknown config key %q", args[0])
		}
		_, err := fmt.Fprintln(application.out, cfg[args[0]])
		return err
	}
	keys := make([]string, 0, len(cfg))
	for key := range cfg {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([][2]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, [2]string{key, cfg[key]})
	}
	return application.writeKV(pairs)
}

func (application *app) runConfigSet(args []string) error {
	if err := requireArgs(args, 2, "composia config set <addr|token_file> <value>"); err != nil {
		return err
	}
	if !isCLIConfigKey(args[0]) {
		return fmt.Errorf("unknown config key %q", args[0])
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}
	cfg[args[0]] = args[1]
	if err := saveCLIConfig(cfg); err != nil {
		return err
	}
	_, err = fmt.Fprintln(application.out, "updated")
	return err
}

func (application *app) runConfigUnset(args []string) error {
	if err := requireArgs(args, 1, "composia config unset <addr|token_file>"); err != nil {
		return err
	}
	if !isCLIConfigKey(args[0]) {
		return fmt.Errorf("unknown config key %q", args[0])
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}
	delete(cfg, args[0])
	if err := saveCLIConfig(cfg); err != nil {
		return err
	}
	_, err = fmt.Fprintln(application.out, "updated")
	return err
}

func (application *app) runConfigPath(args []string) error {
	if err := requireArgs(args, 0, "composia config path"); err != nil {
		return err
	}
	path, err := cliConfigPath()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(application.out, path)
	return err
}

func loadCLIConfig() (cliConfig, error) {
	path, err := cliConfigPath()
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cliConfig{}, nil
		}
		return nil, fmt.Errorf("read CLI config %q: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	return parseCLIConfig(file)
}

func parseCLIConfig(r io.Reader) (cliConfig, error) {
	cfg := cliConfig{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("invalid CLI config line %q", line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !isCLIConfigKey(key) {
			return nil, fmt.Errorf("unknown CLI config key %q", key)
		}
		cfg[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func saveCLIConfig(cfg cliConfig) error {
	path, err := cliConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create CLI config directory: %w", err)
	}
	keys := make([]string, 0, len(cfg))
	for key := range cfg {
		if isCLIConfigKey(key) && cfg[key] != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		b.WriteString(key)
		b.WriteString("=")
		b.WriteString(cfg[key])
		b.WriteString("\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func cliConfigPath() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "composia", "config"), nil
}

func isCLIConfigKey(key string) bool {
	switch key {
	case cliConfigKeyAddr, cliConfigKeyTokenFile:
		return true
	default:
		return false
	}
}
