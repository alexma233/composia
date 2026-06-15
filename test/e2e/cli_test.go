//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	defaultControllerAddr = "http://127.0.0.1:7001"
	defaultAccessToken    = "dev-admin-token"
)

type cliResult struct {
	stdout string
	stderr string
	err    error
}

func TestCLIConnectsToRealController(t *testing.T) {
	bin := buildCLI(t)
	env := cliEnv(t)

	waitForController(t, bin, env)

	status := runCLI(t, bin, env, "--output", "json", "system", "status")
	if status.err != nil {
		t.Fatalf("system status failed: %v\nstdout:\n%s\nstderr:\n%s", status.err, status.stdout, status.stderr)
	}
	statusJSON := decodeJSON(t, status.stdout)
	if version, ok := statusJSON["version"].(string); !ok || version == "" {
		t.Fatalf("expected system status version, got:\n%s", status.stdout)
	}

	nodes := runCLI(t, bin, env, "--output", "json", "node", "list")
	if nodes.err != nil {
		t.Fatalf("node list failed: %v\nstdout:\n%s\nstderr:\n%s", nodes.err, nodes.stdout, nodes.stderr)
	}
	nodesJSON := decodeJSON(t, nodes.stdout)
	if !hasNodeID(nodesJSON, "main") {
		t.Fatalf("expected node list to include main, got:\n%s", nodes.stdout)
	}
}

func buildCLI(t *testing.T) string {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "composia")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", bin, "./cmd/composia")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build CLI: %v\n%s", err, out)
	}

	return bin
}

func repoRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func cliEnv(t *testing.T) []string {
	t.Helper()

	addr := strings.TrimSpace(os.Getenv("COMPOSIA_E2E_CONTROLLER_ADDR"))
	if addr == "" {
		addr = defaultControllerAddr
	}
	token := strings.TrimSpace(os.Getenv("COMPOSIA_E2E_ACCESS_TOKEN"))
	if token == "" {
		token = defaultAccessToken
	}

	home := t.TempDir()
	env := append(os.Environ(),
		"COMPOSIA_CONTROLLER_ADDR="+addr,
		"COMPOSIA_ACCESS_TOKEN="+token,
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
	)
	return env
}

func waitForController(t *testing.T, bin string, env []string) {
	t.Helper()

	deadline := time.Now().Add(60 * time.Second)
	var last cliResult
	for time.Now().Before(deadline) {
		last = runCLI(t, bin, env, "--output", "json", "system", "status")
		if last.err == nil {
			return
		}
		time.Sleep(2 * time.Second)
	}

	t.Fatalf("controller did not become ready: %v\nstdout:\n%s\nstderr:\n%s", last.err, last.stdout, last.stderr)
}

func runCLI(t *testing.T, bin string, env []string, args ...string) cliResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() != nil {
		err = fmt.Errorf("CLI timed out: %w", ctx.Err())
	}
	return cliResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func decodeJSON(t *testing.T, input string) map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal([]byte(input), &decoded); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, input)
	}
	return decoded
}

func hasNodeID(input map[string]any, nodeID string) bool {
	nodes, ok := input["nodes"].([]any)
	if !ok {
		return false
	}
	for _, item := range nodes {
		node, ok := item.(map[string]any)
		if ok && node["node_id"] == nodeID {
			return true
		}
	}
	return false
}
