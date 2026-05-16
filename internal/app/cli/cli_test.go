package cli

import (
	"bytes"
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/version"
)

func TestParseGlobalFlags(t *testing.T) {
	cfg, rest, err := parseGlobalFlags([]string{"--addr", "http://127.0.0.1:7001", "--token", "secret", "--json", "service", "list"})
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
	if err := Run(context.Background(), []string{"service", "deploy", "--help"}, &out, &errOut); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(out.String(), "usage: composia service deploy") {
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
		"task list|get|logs|wait|run-again|approve|reject",
		"node list|get|tasks|stats|sync-caddy-files|reload-caddy|prune",
		"service list|get|workspace|update-candidates|deploy|update|stop|restart|backup|dns-update|caddy-sync|migrate",
		"repo head|files|get|edit|update|mkdir|mv|rm|history|sync|validate",
		"network list|get|remove",
		"volume list|get|remove",
		"image list|get|remove",
		"rustic init|forget|prune",
		"container list|get|logs|start|stop|restart|remove|exec",
		"skills list|show",
		"completion bash|zsh|fish",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage missing %q:\n%s", want, usage)
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
		{[]string{"help", "service", "workspace"}, "usage: composia service workspace <list|get>"},
		{[]string{"help", "service", "workspace", "list"}, "usage: composia service workspace list"},
		{[]string{"help", "service", "workspace", "get"}, "usage: composia service workspace get <folder>"},
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

func TestServiceActionUsageMatchesAction(t *testing.T) {
	err := (&app{}).runServiceAction("backup", nil)
	if err == nil {
		t.Fatalf("expected usage error")
	}
	message := err.Error()
	if !strings.Contains(message, "usage: composia service backup") {
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
	cfg, err := parseCLIConfig(strings.NewReader("addr=https://controller.example\ntoken_file=/run/secrets/composia\n"))
	if err != nil {
		t.Fatalf("parseCLIConfig returned error: %v", err)
	}
	if cfg[cliConfigKeyAddr] != "https://controller.example" {
		t.Fatalf("addr = %q", cfg[cliConfigKeyAddr])
	}
	if cfg[cliConfigKeyTokenFile] != "/run/secrets/composia" {
		t.Fatalf("token_file = %q", cfg[cliConfigKeyTokenFile])
	}
}

func TestRepoWriteErrorAddsConflictHint(t *testing.T) {
	err := connect.NewError(connect.CodeFailedPrecondition, errors.New(`base_revision "old" does not match current HEAD "new"`))
	got := repoWriteError(err).Error()
	if !strings.Contains(got, "repo changed while preparing this write") {
		t.Fatalf("error = %q", got)
	}
}
