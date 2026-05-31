package cli

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/version"
)

func TestParseGlobalFlags(t *testing.T) {
	cfg, rest, err := parseGlobalFlags([]string{"--addr", "http://127.0.0.1:7001", "--token", "secret", "--header", "CF-Access-Client-Id: id", "--json", "service", "list"})
	if err != nil {
		t.Fatalf("parseGlobalFlags returned error: %v", err)
	}
	if cfg.addr != "http://127.0.0.1:7001" {
		t.Fatalf("addr = %q", cfg.addr)
	}
	if cfg.token != "secret" {
		t.Fatalf("token = %q", cfg.token)
	}
	if !cfg.json {
		t.Fatalf("json = false")
	}
	if got := cfg.headers["Cf-Access-Client-Id"]; got != "id" {
		t.Fatalf("header = %q", got)
	}
	if cfg.output != outputModeJSON {
		t.Fatalf("output = %q", cfg.output)
	}
	if strings.Join(rest, " ") != "service list" {
		t.Fatalf("rest = %v", rest)
	}
}

func TestParseGlobalFlagsTerse(t *testing.T) {
	cfg, rest, err := parseGlobalFlags([]string{"--output", "json", "--terse", "service", "list"})
	if err != nil {
		t.Fatalf("parseGlobalFlags returned error: %v", err)
	}
	if cfg.output != outputModeTerse || !cfg.terse || cfg.json {
		t.Fatalf("cfg = %+v", cfg)
	}
	if strings.Join(rest, " ") != "service list" {
		t.Fatalf("rest = %v", rest)
	}
}

func TestParseGlobalFlagsRejectsUnknownOutput(t *testing.T) {
	if _, _, err := parseGlobalFlags([]string{"--output", "xml", "service", "list"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseGlobalFlagsRejectsReservedHeader(t *testing.T) {
	if _, _, err := parseGlobalFlags([]string{"--header", "Authorization: Bearer token", "service", "list"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseGlobalFlagsRejectsTokenAndTokenFile(t *testing.T) {
	if _, _, err := parseGlobalFlags([]string{"--token", "secret", "--token-file", "/tmp/token", "service", "list"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestStringListFlag(t *testing.T) {
	var values stringListFlag
	if err := values.Set("main, edge"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := values.Set("worker"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	got := strings.Join([]string(values), ",")
	if got != "main,edge,worker" {
		t.Fatalf("values = %q", got)
	}
}

func TestIsControllerCommandIncludesSecret(t *testing.T) {
	if !isControllerCommand("secret") {
		t.Fatalf("secret command is not recognized")
	}
	for _, command := range []string{"network", "volume", "image", "rustic"} {
		if !isControllerCommand(command) {
			t.Fatalf("%s command is not recognized", command)
		}
	}
	if isControllerCommand("controller") {
		t.Fatalf("controller should stay outside controller RPC command set")
	}
}

func TestChooseEditor(t *testing.T) {
	t.Setenv("VISUAL", "code --wait")
	t.Setenv("EDITOR", "vim")
	if got := chooseEditor(); got != "code --wait" {
		t.Fatalf("chooseEditor with VISUAL = %q", got)
	}

	t.Setenv("VISUAL", "")
	if got := chooseEditor(); got != "vim" {
		t.Fatalf("chooseEditor with EDITOR = %q", got)
	}

	t.Setenv("EDITOR", "")
	if got := chooseEditor(); got != "vi" {
		t.Fatalf("chooseEditor fallback = %q", got)
	}
}

func TestServiceActionFromName(t *testing.T) {
	action, err := serviceActionFromName("dns-update")
	if err != nil {
		t.Fatalf("serviceActionFromName returned error: %v", err)
	}
	if action != controllerv1.ServiceAction_SERVICE_ACTION_DNS_UPDATE {
		t.Fatalf("action = %v", action)
	}
	if _, err := serviceActionFromName("bad"); err == nil {
		t.Fatalf("expected error for unknown action")
	}
}

func TestImageUpdateSelectionsParsesDetectedAndManualUpdates(t *testing.T) {
	selections, err := imageUpdateSelections([]string{"api"}, []string{"web=2026.05.08"}, true)
	if err != nil {
		t.Fatalf("imageUpdateSelections returned error: %v", err)
	}
	if len(selections) != 2 {
		t.Fatalf("expected 2 selections, got %+v", selections)
	}
	if selections[0].GetImageName() != "api" || !selections[0].GetUseDetected() {
		t.Fatalf("unexpected detected selection: %+v", selections[0])
	}
	if selections[1].GetImageName() != "web" || selections[1].GetTargetTag() != "2026.05.08" {
		t.Fatalf("unexpected manual selection: %+v", selections[1])
	}
}

func TestImageUpdateSelectionsRejectsDuplicateUpdates(t *testing.T) {
	if _, err := imageUpdateSelections([]string{"api"}, []string{"api=1.2.3"}, true); err == nil {
		t.Fatalf("expected duplicate image update error")
	}
}

func TestContainerActionFromName(t *testing.T) {
	action, err := containerActionFromName("restart")
	if err != nil {
		t.Fatalf("containerActionFromName returned error: %v", err)
	}
	if action != controllerv1.ContainerAction_CONTAINER_ACTION_RESTART {
		t.Fatalf("action = %v", action)
	}
	if _, err := containerActionFromName("bad"); err == nil {
		t.Fatalf("expected error for unknown action")
	}
}

func TestWriteTable(t *testing.T) {
	var out bytes.Buffer
	if err := writeTable(&out, []string{"NAME", "STATUS"}, [][]string{{"alpha", "running"}}); err != nil {
		t.Fatalf("writeTable returned error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "NAME") || !strings.Contains(got, "alpha") {
		t.Fatalf("unexpected table output: %q", got)
	}
}

func TestAppTerseOutput(t *testing.T) {
	var out bytes.Buffer
	application := &app{out: &out, cfg: globalConfig{output: outputModeTerse, terse: true}}
	if err := application.writeTable([]string{"NAME", "STATUS"}, [][]string{{"alpha", "running"}}); err != nil {
		t.Fatalf("writeTable returned error: %v", err)
	}
	if err := application.writeKV([][2]string{{"task_id", "tsk_1"}, {"empty", ""}}); err != nil {
		t.Fatalf("writeKV returned error: %v", err)
	}
	if err := application.writeCount("total_count", 1); err != nil {
		t.Fatalf("writeCount returned error: %v", err)
	}
	got := out.String()
	want := "alpha running\ntask_id=tsk_1\n"
	if got != want {
		t.Fatalf("terse output = %q, want %q", got, want)
	}
}

func TestRunVersionDoesNotRequireControllerConfig(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run(context.Background(), []string{"version"}, &out, &errOut); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if strings.TrimSpace(out.String()) != version.Value {
		t.Fatalf("version output = %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestRunUnknownCommandDoesNotRequireControllerConfig(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	err := Run(context.Background(), []string{"missing"}, &out, &errOut)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), `unknown command "missing"`) {
		t.Fatalf("error = %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q", out.String())
	}
	if !strings.Contains(errOut.String(), "usage: composia") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestHelpSubcommandDoesNotRequireControllerConfig(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run(context.Background(), []string{"service", "vaultwarden", "up", "--help"}, &out, &errOut); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(out.String(), "usage: composia service vaultwarden up") {
		t.Fatalf("stdout = %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestUsageIncludesWaitAndNewCommands(t *testing.T) {
	var out bytes.Buffer
	PrintUsage(&out)
	usage := out.String()
	for _, want := range []string{
		"service     List/create services or target one service by name",
		"task        Inspect task status",
		"node        Inspect nodes",
		"container   Low-level container operations",
		"repo        Low-level repository file operations",
		"secret      Low-level encrypted file operations",
		"system      Controller status",
		"completion  Generate shell completion scripts",
		"Run 'composia help <command>' for command details.",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage missing %q:\n%s", want, usage)
		}
	}
}

func TestPrintSystemCapabilities(t *testing.T) {
	var out bytes.Buffer
	application := &app{out: &out}
	message := &controllerv1.GetCapabilitiesResponse{Global: &controllerv1.GlobalCapabilities{
		Backup: &controllerv1.Capability{Enabled: true},
		Dns: &controllerv1.Capability{
			Enabled:    false,
			ReasonCode: controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_MISSING_DNS_INTEGRATION,
		},
		Secrets:           &controllerv1.Capability{Enabled: true},
		RusticMaintenance: &controllerv1.Capability{Enabled: false},
	}}

	if err := application.printSystemCapabilities(message); err != nil {
		t.Fatalf("printSystemCapabilities returned error: %v", err)
	}
	got := out.String()
	for _, want := range []string{"CAPABILITY", "backup", "true", "dns", "false", "missing_dns_integration", "rustic_maintenance"} {
		if !strings.Contains(got, want) {
			t.Fatalf("system capabilities output missing %q:\n%s", want, got)
		}
	}
}

func TestExecCLIHelpers(t *testing.T) {
	got, err := durationSeconds(1500)
	if err != nil || got != 1 {
		t.Fatalf("durationSeconds short duration = %d", got)
	}
	got, err = durationSeconds(1500_000_000)
	if err != nil || got != 2 {
		t.Fatalf("durationSeconds rounded duration = %d", got)
	}
	if _, err := durationSeconds((time.Duration(math.MaxUint32) + 1) * time.Second); err == nil {
		t.Fatalf("expected duration overflow error")
	}
	if _, err := uint32FlagValue("page-size", uint(math.MaxUint32)+1); err == nil {
		t.Fatalf("expected uint32 flag overflow error")
	}
	origin, err := controllerOrigin("https://Controller.Example:8443/base")
	if err != nil {
		t.Fatalf("controllerOrigin returned error: %v", err)
	}
	if origin != "https://controller.example:8443" {
		t.Fatalf("origin = %q", origin)
	}
	wsURL, err := containerExecWebsocketURL("https://controller.example/base", rpcutil.ControllerExecWSPath+"token")
	if err != nil {
		t.Fatalf("containerExecWebsocketURL returned error: %v", err)
	}
	if wsURL != "wss://controller.example/api/controller/ws/container-exec/token" {
		t.Fatalf("wsURL = %q", wsURL)
	}
	done, err := handleExecWebsocketEvent([]byte(`{"type":"closed"}`))
	if err != nil || !done {
		t.Fatalf("closed event done=%v err=%v", done, err)
	}
	if _, err := handleExecWebsocketEvent([]byte(`{`)); err == nil || !strings.Contains(err.Error(), "invalid container exec websocket event JSON") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
	if _, err := handleExecWebsocketEvent([]byte(`{"type":"mystery"}`)); err == nil || !strings.Contains(err.Error(), `unknown container exec websocket event type "mystery"`) {
		t.Fatalf("expected unknown event type error, got %v", err)
	}
}

func TestHelpUnknownTopicReturnsError(t *testing.T) {
	var out bytes.Buffer
	err := Run(context.Background(), []string{"help", "missing"}, &out, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), `unknown help topic "missing"`) {
		t.Fatalf("expected unknown help topic error, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestNewCLICommandHelpDoesNotRequireControllerConfig(t *testing.T) {
	for _, testCase := range []struct {
		args []string
		want string
	}{
		{[]string{"help", "service", "create"}, "usage: composia service create [--message text] <name>"},
		{[]string{"help", "service", "vaultwarden"}, "usage: composia service vaultwarden [--containers]"},
		{[]string{"help", "service", "vaultwarden", "edit"}, "usage: composia service vaultwarden edit [--message text] <compose|meta|env|path>"},
		{[]string{"help", "service", "vaultwarden", "exec"}, "usage: composia service vaultwarden exec [--node node] [--container name] [--no-tty] [command] [args...]"},
		{[]string{"help", "repo", "mkdir"}, "usage: composia repo mkdir [--message text] <path>"},
		{[]string{"help", "repo", "mv"}, "usage: composia repo mv [--message text] <source> <destination>"},
		{[]string{"help", "repo", "rm"}, "usage: composia repo rm [--message text] <path>"},
		{[]string{"help", "node", "sync-caddy-files"}, "usage: composia node sync-caddy-files [--wait] [--follow] [--timeout duration] [--service name] [--full-rebuild] <node>"},
	} {
		var out bytes.Buffer
		var errOut bytes.Buffer
		if err := Run(context.Background(), testCase.args, &out, &errOut); err != nil {
			t.Fatalf("Run(%v) returned error: %v", testCase.args, err)
		}
		if !strings.Contains(out.String(), testCase.want) {
			t.Fatalf("Run(%v) stdout = %q, want %q", testCase.args, out.String(), testCase.want)
		}
		if errOut.Len() != 0 {
			t.Fatalf("Run(%v) stderr = %q", testCase.args, errOut.String())
		}
	}
}

func TestConfigSetRejectsMultilineValue(t *testing.T) {
	var out bytes.Buffer
	application := &app{out: &out}
	err := application.runConfigSet([]string{"addr", "https://controller.example\nnext"})
	if err == nil || !strings.Contains(err.Error(), "must not contain newline") {
		t.Fatalf("expected config value validation error, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestCLIConfigPathUsesXDGCLIConfig(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	path, err := cliConfigPath()
	if err != nil {
		t.Fatalf("cliConfigPath returned error: %v", err)
	}
	want := filepath.Join(configHome, "composia", "cli", "config")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	tokenPath, err := cliTokenPath()
	if err != nil {
		t.Fatalf("cliTokenPath returned error: %v", err)
	}
	if tokenPath != filepath.Join(configHome, "composia", "cli", "token") {
		t.Fatalf("token path = %q", tokenPath)
	}
}

func TestConfigureClientUsesConfigBeforeEnv(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv(envControllerAddr, "https://env-controller.example")
	t.Setenv(envAccessToken, "env-token")
	tokenPath := filepath.Join(configHome, "composia", "cli", "token")
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(tokenPath, []byte("config-token\n"), 0o600); err != nil {
		t.Fatalf("WriteFile token returned error: %v", err)
	}
	if err := saveCLIConfig(cliConfig{cliConfigKeyAddr: "https://config-controller.example", cliConfigKeyTokenFile: tokenPath}); err != nil {
		t.Fatalf("saveCLIConfig returned error: %v", err)
	}
	application := &app{cfg: globalConfig{headers: map[string]string{}}}
	if err := application.configureClient(); err != nil {
		t.Fatalf("configureClient returned error: %v", err)
	}
	if application.cfg.addr != "https://config-controller.example" {
		t.Fatalf("addr = %q", application.cfg.addr)
	}
	if application.cfg.token != "config-token" {
		t.Fatalf("token = %q", application.cfg.token)
	}
}

func TestConfigureClientReadsConfigKeyringBeforeEnv(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv(envControllerAddr, "https://env-controller.example")
	t.Setenv(envAccessToken, "env-token")
	secrets := stubCLIKeyring(t)
	secrets[cliKeyringAccount("default")] = "keyring-token"
	if err := saveCLIConfig(cliConfig{cliConfigKeyAddr: "https://config-controller.example", cliConfigKeyTokenKeyring: defaultCLIKeyringName}); err != nil {
		t.Fatalf("saveCLIConfig returned error: %v", err)
	}
	application := &app{cfg: globalConfig{headers: map[string]string{}}}
	if err := application.configureClient(); err != nil {
		t.Fatalf("configureClient returned error: %v", err)
	}
	if application.cfg.token != "keyring-token" {
		t.Fatalf("token = %q", application.cfg.token)
	}
}

func TestParseCLIConfigRejectsTokenAndTokenFile(t *testing.T) {
	_, err := parseCLIConfig(strings.NewReader("token=inline\ntoken_file=/run/secrets/composia\ntoken_keyring=default\n"))
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got %v", err)
	}
}

func TestConfigGetRedactsInlineToken(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	if err := saveCLIConfig(cliConfig{cliConfigKeyToken: "secret"}); err != nil {
		t.Fatalf("saveCLIConfig returned error: %v", err)
	}
	var out bytes.Buffer
	application := &app{out: &out, cfg: globalConfig{output: outputModeHuman}}
	if err := application.runConfigGet([]string{cliConfigKeyToken}); err != nil {
		t.Fatalf("runConfigGet returned error: %v", err)
	}
	if strings.TrimSpace(out.String()) != redactedSecretValue {
		t.Fatalf("token output = %q", out.String())
	}
}

func TestConfigSetTokenStdinWritesDefaultKeyring(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	secrets := stubCLIKeyring(t)
	stdin := replaceStdin(t, "secret-token\n")
	defer stdin()
	var out bytes.Buffer
	application := &app{out: &out}
	if err := application.runConfigSetToken([]string{"--stdin"}); err != nil {
		t.Fatalf("runConfigSetToken returned error: %v", err)
	}
	if secrets[cliKeyringAccount("default")] != "secret-token" {
		t.Fatalf("keyring token = %q", secrets[cliKeyringAccount("default")])
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		t.Fatalf("loadCLIConfig returned error: %v", err)
	}
	if cfg[cliConfigKeyTokenKeyring] != defaultCLIKeyringName || cfg[cliConfigKeyTokenFile] != "" || cfg[cliConfigKeyToken] != "" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestConfigSetTokenFileWritesDefaultTokenFile(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	stdin := replaceStdin(t, "secret-token\n")
	defer stdin()
	var out bytes.Buffer
	application := &app{out: &out}
	if err := application.runConfigSetToken([]string{"--stdin", "--file"}); err != nil {
		t.Fatalf("runConfigSetToken returned error: %v", err)
	}
	tokenPath, err := cliTokenPath()
	if err != nil {
		t.Fatalf("cliTokenPath returned error: %v", err)
	}
	content, err := os.ReadFile(tokenPath) //nolint:gosec
	if err != nil {
		t.Fatalf("ReadFile token returned error: %v", err)
	}
	if string(content) != "secret-token\n" {
		t.Fatalf("token file = %q", content)
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		t.Fatalf("loadCLIConfig returned error: %v", err)
	}
	if cfg[cliConfigKeyTokenFile] != tokenPath || cfg[cliConfigKeyToken] != "" || cfg[cliConfigKeyTokenKeyring] != "" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestConfigSetTokenRejectsFileAndInline(t *testing.T) {
	var out bytes.Buffer
	application := &app{out: &out}
	err := application.runConfigSetToken([]string{"--file", "--inline"})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got %v", err)
	}
}

func TestConfigUnsetTokenDeletesKeyringToken(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	secrets := stubCLIKeyring(t)
	secrets[cliKeyringAccount("default")] = "secret-token"
	if err := saveCLIConfig(cliConfig{cliConfigKeyTokenKeyring: defaultCLIKeyringName}); err != nil {
		t.Fatalf("saveCLIConfig returned error: %v", err)
	}
	var out bytes.Buffer
	application := &app{out: &out}
	if err := application.runConfigUnsetToken(nil); err != nil {
		t.Fatalf("runConfigUnsetToken returned error: %v", err)
	}
	if _, ok := secrets[cliKeyringAccount("default")]; ok {
		t.Fatalf("keyring token was not deleted")
	}
}

func TestServiceActionUsageMatchesAction(t *testing.T) {
	err := (&app{}).runServiceAction("backup", "vaultwarden", []string{"extra"})
	if err == nil {
		t.Fatalf("expected usage error")
	}
	message := err.Error()
	if !strings.Contains(message, "usage: composia service <service> backup") {
		t.Fatalf("usage = %q", message)
	}
	if strings.Contains(message, "--recreate") || strings.Contains(message, "--image") {
		t.Fatalf("backup usage drifted: %q", message)
	}
}

func TestCompletionDoesNotRequireControllerConfig(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run(context.Background(), []string{"completion", "bash"}, &out, &errOut); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	got := out.String()
	for _, want := range []string{"complete -F _composia_completion composia", "network", "rustic"} {
		if !strings.Contains(got, want) {
			t.Fatalf("completion missing %q:\n%s", want, got)
		}
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestSkillsDoesNotRequireControllerConfig(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run(context.Background(), []string{"--terse", "skills", "list"}, &out, &errOut); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "coding-agent ") {
		t.Fatalf("skills list missing coding-agent:\n%s", got)
	}
	if strings.Contains(got, "SKILL") {
		t.Fatalf("terse skills list included header:\n%s", got)
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestSkillsShowDoesNotRequireControllerConfig(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run(context.Background(), []string{"skills", "show", "coding-agent"}, &out, &errOut); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(out.String(), "Use --terse") {
		t.Fatalf("skills show output = %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestParseCLIConfig(t *testing.T) {
	cfg, err := parseCLIConfig(strings.NewReader("addr=https://controller.example\ntoken_keyring=default\n"))
	if err != nil {
		t.Fatalf("parseCLIConfig returned error: %v", err)
	}
	if cfg[cliConfigKeyAddr] != "https://controller.example" {
		t.Fatalf("addr = %q", cfg[cliConfigKeyAddr])
	}
	if cfg[cliConfigKeyTokenKeyring] != "default" {
		t.Fatalf("token_keyring = %q", cfg[cliConfigKeyTokenKeyring])
	}
}

func stubCLIKeyring(t *testing.T) map[string]string {
	t.Helper()
	secrets := map[string]string{}
	old := cliKeyring
	cliKeyring = keyringBackend{
		Get: func(service, user string) (string, error) {
			secret, ok := secrets[user]
			if !ok {
				return "", errors.New("not found")
			}
			return secret, nil
		},
		Set: func(service, user, password string) error {
			secrets[user] = password
			return nil
		},
		Delete: func(service, user string) error {
			delete(secrets, user)
			return nil
		},
	}
	t.Cleanup(func() { cliKeyring = old })
	return secrets
}

func replaceStdin(t *testing.T, content string) func() {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stdin-*")
	if err != nil {
		t.Fatalf("CreateTemp returned error: %v", err)
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		t.Fatalf("WriteString returned error: %v", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		_ = file.Close()
		t.Fatalf("Seek returned error: %v", err)
	}
	old := os.Stdin
	os.Stdin = file
	return func() {
		os.Stdin = old
		_ = file.Close()
	}
}

func TestRepoWriteErrorAddsConflictHint(t *testing.T) {
	err := connect.NewError(connect.CodeFailedPrecondition, errors.New(`base_revision "old" does not match current HEAD "new"`))
	got := repoWriteError(err).Error()
	if !strings.Contains(got, "repo changed while preparing this write") {
		t.Fatalf("error = %q", got)
	}
}
