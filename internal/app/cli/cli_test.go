package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
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
	if strings.Join(rest, " ") != "service list" {
		t.Fatalf("rest = %v", rest)
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

func TestInstanceActionFromName(t *testing.T) {
	action, err := instanceActionFromName("restart")
	if err != nil {
		t.Fatalf("instanceActionFromName returned error: %v", err)
	}
	if action != controllerv1.ServiceInstanceAction_SERVICE_INSTANCE_ACTION_RESTART {
		t.Fatalf("action = %v", action)
	}
	if _, err := instanceActionFromName("backup"); err == nil {
		t.Fatalf("expected error for unsupported direct instance action")
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
