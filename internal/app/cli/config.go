package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	keyring "github.com/zalando/go-keyring"
	"golang.org/x/term"
)

const (
	cliConfigKeyAddr         = "addr"
	cliConfigKeyToken        = "token"
	cliConfigKeyTokenFile    = "token_file"
	cliConfigKeyTokenKeyring = "token_keyring"
	defaultCLIKeyringName    = "default"
	cliKeyringService        = "composia"
	redactedSecretValue      = "<redacted>"
)

var cliKeyring = keyringBackend{
	Get:    keyring.Get,
	Set:    keyring.Set,
	Delete: keyring.Delete,
}

type keyringBackend struct {
	Get    func(service, user string) (string, error)
	Set    func(service, user, password string) error
	Delete func(service, user string) error
}

type cliConfig map[string]string

func (application *app) runConfigGet(args []string) error {
	if len(args) > 1 {
		return errors.New("usage: composia config get [key]")
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}
	if len(args) == 1 {
		if !isCLIConfigKey(args[0]) {
			return fmt.Errorf("unknown config key %q", args[0])
		}
		_, err := fmt.Fprintln(application.out, printableCLIConfigValue(args[0], cfg[args[0]]))
		return err
	}
	keys := make([]string, 0, len(cfg))
	for key := range cfg {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([][2]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, [2]string{key, printableCLIConfigValue(key, cfg[key])})
	}
	return application.writeKV(pairs)
}

func (application *app) runConfigSet(args []string) error {
	if err := requireArgs(args, 2, "composia config set <addr|token_file|token_keyring> <value>"); err != nil {
		return err
	}
	if args[0] == cliConfigKeyToken {
		return errors.New("use composia config set-token to store controller access tokens")
	}
	if !isCLIConfigKey(args[0]) {
		return fmt.Errorf("unknown config key %q", args[0])
	}
	if strings.ContainsAny(args[1], "\n\r\x00") {
		return fmt.Errorf("config value for %q must not contain newline, carriage return, or NUL", args[0])
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}
	cfg[args[0]] = args[1]
	if args[0] == cliConfigKeyTokenFile {
		delete(cfg, cliConfigKeyToken)
		delete(cfg, cliConfigKeyTokenKeyring)
	}
	if args[0] == cliConfigKeyTokenKeyring {
		delete(cfg, cliConfigKeyToken)
		delete(cfg, cliConfigKeyTokenFile)
	}
	if err := saveCLIConfig(cfg); err != nil {
		return err
	}
	_, err = fmt.Fprintln(application.out, "updated")
	return err
}

func (application *app) runConfigUnset(args []string) error {
	if err := requireArgs(args, 1, "composia config unset <addr|token|token_file|token_keyring>"); err != nil {
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

func (application *app) runConfigSetToken(args []string) error {
	fs := newCommandFlagSet("config set-token")
	stdin := fs.Bool("stdin", false, "read token from stdin")
	inline := fs.Bool("inline", false, "store token inline in CLI config")
	file := fs.Bool("file", false, "store token in the default CLI token file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: composia config set-token [--stdin] [--file|--inline]")
	}
	storage, err := cliTokenStorageMode(*file, *inline)
	if err != nil {
		return err
	}
	token, err := readCLISecret("Controller access token", *stdin)
	if err != nil {
		return err
	}
	return application.saveConfigToken(token, storage)
}

func (application *app) runConfigUnsetToken(args []string) error {
	if err := requireArgs(args, 0, "composia config unset-token"); err != nil {
		return err
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}
	tokenFile := cfg[cliConfigKeyTokenFile]
	tokenKeyring := cfg[cliConfigKeyTokenKeyring]
	delete(cfg, cliConfigKeyToken)
	delete(cfg, cliConfigKeyTokenFile)
	delete(cfg, cliConfigKeyTokenKeyring)
	if err := saveCLIConfig(cfg); err != nil {
		return err
	}
	defaultTokenPath, err := cliTokenPath()
	if err != nil {
		return err
	}
	if tokenFile == defaultTokenPath {
		if err := os.Remove(defaultTokenPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove CLI token file %q: %w", defaultTokenPath, err)
		}
	}
	if tokenKeyring != "" {
		if err := deleteCLIKeyringToken(tokenKeyring); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return fmt.Errorf("delete CLI token from system keyring: %w", err)
		}
	}
	_, err = fmt.Fprintln(application.out, "updated")
	return err
}

func (application *app) runConfigSetup(args []string) error {
	fs := newCommandFlagSet("config setup")
	stdin := fs.Bool("stdin", false, "read token from stdin")
	inline := fs.Bool("inline", false, "store token inline in CLI config")
	file := fs.Bool("file", false, "store token in the default CLI token file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: composia config setup [--stdin] [--file|--inline]")
	}
	storage, err := cliTokenStorageMode(*file, *inline)
	if err != nil {
		return err
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}
	path, err := cliConfigPath()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(application.out, "CLI config: %s\n", path)
	addr, err := promptCLIValue(application.out, "Controller address", cfg[cliConfigKeyAddr])
	if err != nil {
		return err
	}
	if strings.TrimSpace(addr) != "" {
		cfg[cliConfigKeyAddr] = strings.TrimSpace(addr)
	}
	token, err := readCLISecret("Controller access token", *stdin)
	if err != nil {
		return err
	}
	if err := saveConfigTokenToConfig(cfg, token, storage); err != nil {
		return err
	}
	if err := saveCLIConfig(cfg); err != nil {
		return err
	}
	_, err = fmt.Fprintln(application.out, "updated")
	return err
}

func (application *app) saveConfigToken(token string, storage cliTokenStorage) error {
	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}
	if err := saveConfigTokenToConfig(cfg, token, storage); err != nil {
		return err
	}
	if err := saveCLIConfig(cfg); err != nil {
		return err
	}
	_, err = fmt.Fprintln(application.out, "updated")
	return err
}

type cliTokenStorage string

const (
	cliTokenStorageKeyring cliTokenStorage = "keyring"
	cliTokenStorageFile    cliTokenStorage = "file"
	cliTokenStorageInline  cliTokenStorage = "inline"
)

func cliTokenStorageMode(file bool, inline bool) (cliTokenStorage, error) {
	if file && inline {
		return "", errors.New("--file and --inline are mutually exclusive")
	}
	if inline {
		return cliTokenStorageInline, nil
	}
	if file {
		return cliTokenStorageFile, nil
	}
	return cliTokenStorageKeyring, nil
}

func saveConfigTokenToConfig(cfg cliConfig, token string, storage cliTokenStorage) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("controller access token must not be empty")
	}
	delete(cfg, cliConfigKeyToken)
	delete(cfg, cliConfigKeyTokenFile)
	delete(cfg, cliConfigKeyTokenKeyring)
	switch storage {
	case cliTokenStorageInline:
		cfg[cliConfigKeyToken] = token
	case cliTokenStorageFile:
		path, err := cliTokenPath()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("create CLI config directory: %w", err)
		}
		if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
			return fmt.Errorf("write CLI token file %q: %w", path, err)
		}
		cfg[cliConfigKeyTokenFile] = path
	case cliTokenStorageKeyring:
		if err := saveCLIKeyringToken(defaultCLIKeyringName, token); err != nil {
			return fmt.Errorf("store token in system keyring failed: %w. Use --file to store the token at %s instead", err, mustCLITokenPathForError())
		}
		cfg[cliConfigKeyTokenKeyring] = defaultCLIKeyringName
	default:
		return fmt.Errorf("unknown token storage mode %q", storage)
	}
	return nil
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
	file, err := os.Open(path) //nolint:gosec
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
	if err := validateCLITokenSourceConfig(cfg); err != nil {
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
	dir, err := cliConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config"), nil
}

func cliTokenPath() (string, error) {
	dir, err := cliConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "token"), nil
}

func cliConfigDir() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "composia", "cli"), nil
}

func isCLIConfigKey(key string) bool {
	switch key {
	case cliConfigKeyAddr, cliConfigKeyToken, cliConfigKeyTokenFile, cliConfigKeyTokenKeyring:
		return true
	default:
		return false
	}
}

func validateCLITokenSourceConfig(cfg cliConfig) error {
	keys := make([]string, 0, 3)
	for _, key := range []string{cliConfigKeyToken, cliConfigKeyTokenFile, cliConfigKeyTokenKeyring} {
		if strings.TrimSpace(cfg[key]) != "" {
			keys = append(keys, key)
		}
	}
	if len(keys) > 1 {
		return fmt.Errorf("CLI config keys %s are mutually exclusive", strings.Join(keys, ", "))
	}
	return nil
}

func readCLIKeyringToken(name string) (string, error) {
	return cliKeyring.Get(cliKeyringService, cliKeyringAccount(name))
}

func saveCLIKeyringToken(name string, token string) error {
	return cliKeyring.Set(cliKeyringService, cliKeyringAccount(name), token)
}

func deleteCLIKeyringToken(name string) error {
	return cliKeyring.Delete(cliKeyringService, cliKeyringAccount(name))
}

func cliKeyringAccount(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultCLIKeyringName
	}
	return "cli:" + name + ":controller-token"
}

func mustCLITokenPathForError() string {
	path, err := cliTokenPath()
	if err != nil {
		return "the default CLI token file"
	}
	return path
}

func printableCLIConfigValue(key string, value string) string {
	if key == cliConfigKeyToken && value != "" {
		return redactedSecretValue
	}
	return value
}

func readCLISecret(prompt string, stdin bool) (string, error) {
	if stdin {
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read token from stdin: %w", err)
		}
		return strings.TrimSpace(string(content)), nil
	}
	fd := int(os.Stdin.Fd()) //nolint:gosec
	if !term.IsTerminal(fd) {
		return "", errors.New("stdin is not a terminal; pass --stdin to read token from stdin")
	}
	_, _ = fmt.Fprintf(os.Stderr, "%s: ", prompt)
	content, err := term.ReadPassword(fd)
	_, _ = fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read token: %w", err)
	}
	return strings.TrimSpace(string(content)), nil
}

func promptCLIValue(out io.Writer, prompt string, current string) (string, error) {
	fd := int(os.Stdin.Fd()) //nolint:gosec
	if !term.IsTerminal(fd) {
		return current, nil
	}
	if current == "" {
		_, _ = fmt.Fprintf(out, "%s: ", prompt)
	} else {
		_, _ = fmt.Fprintf(out, "%s [%s]: ", prompt, current)
	}
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return current, nil
	}
	value := strings.TrimSpace(scanner.Text())
	if value == "" {
		return current, nil
	}
	return value, nil
}
