package agent

import (
	"path/filepath"
	"strings"
	"testing"

	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
)

func TestCurrentImageUpdateValueFromEnvFile(t *testing.T) {
	t.Parallel()

	serviceDir := t.TempDir()
	writeAgentTestFile(t, filepath.Join(serviceDir, ".env"), "# comment\nAPP_IMAGE='1.2.3@sha256:abc'\n")

	value, tag, digest, err := currentImageUpdateValue(serviceDir, repo.ImageUpdateConfig{Current: repo.ImageUpdateCurrent{Env: &repo.ImageUpdateCurrentEnv{File: ".env", Key: "APP_IMAGE"}}})
	if err != nil {
		t.Fatalf("currentImageUpdateValue returned error: %v", err)
	}
	if value != "1.2.3@sha256:abc" || tag != "1.2.3" || digest != "sha256:abc" {
		t.Fatalf("value/tag/digest = %q/%q/%q", value, tag, digest)
	}
}

func TestCurrentImageUpdateValueFromYAMLPath(t *testing.T) {
	t.Parallel()

	serviceDir := t.TempDir()
	writeAgentTestFile(t, filepath.Join(serviceDir, "compose.yaml"), "services:\n  web:\n    image: ghcr.io/example/web:2.0.0@sha256:def\n")

	value, tag, digest, err := currentImageUpdateValue(serviceDir, repo.ImageUpdateConfig{Current: repo.ImageUpdateCurrent{YAML: &repo.ImageUpdateCurrentYAML{File: "compose.yaml", Path: "services.web.image"}}})
	if err != nil {
		t.Fatalf("currentImageUpdateValue returned error: %v", err)
	}
	if value != "ghcr.io/example/web:2.0.0@sha256:def" || tag != "2.0.0" || digest != "sha256:def" {
		t.Fatalf("value/tag/digest = %q/%q/%q", value, tag, digest)
	}
}

func TestCurrentImageUpdateValueRequiresSource(t *testing.T) {
	t.Parallel()

	value, tag, digest, err := currentImageUpdateValue(t.TempDir(), repo.ImageUpdateConfig{})
	if err == nil || !strings.Contains(err.Error(), "current source is required") {
		t.Fatalf("expected missing source error, got %v", err)
	}
	if value != "" || tag != "" || digest != "" {
		t.Fatalf("expected empty value/tag/digest, got %q/%q/%q", value, tag, digest)
	}
}

func TestEnvFileValue(t *testing.T) {
	t.Parallel()

	value, err := envFileValue(" APP_VERSION = \"2026.05\" \nOTHER=ignored\n", "APP_VERSION")
	if err != nil {
		t.Fatalf("envFileValue returned error: %v", err)
	}
	if value != "2026.05" {
		t.Fatalf("value = %q", value)
	}
	if _, err := envFileValue("APP_VERSION=1", "MISSING"); err == nil {
		t.Fatalf("expected missing key error")
	}
}

func TestYAMLPathStringValue(t *testing.T) {
	t.Parallel()

	value, err := yamlPathStringValue([]byte("a:\n  b:\n    c: value\n"), "a.b.c")
	if err != nil {
		t.Fatalf("yamlPathStringValue returned error: %v", err)
	}
	if value != "value" {
		t.Fatalf("value = %q", value)
	}
	if _, err := yamlPathStringValue([]byte("a:\n  b: []\n"), "a.b.c"); err == nil {
		t.Fatalf("expected non-mapping path error")
	}
}

func TestSplitImageTagDigestHelpers(t *testing.T) {
	t.Parallel()

	if tag, digest := splitTagDigest(" 1.2.3@sha256:abc "); tag != "1.2.3" || digest != "sha256:abc" {
		t.Fatalf("splitTagDigest = %q/%q", tag, digest)
	}
	if tag, digest := splitImageRefTagDigest("registry:5000/ns/app:1.2.3@sha256:def"); tag != "1.2.3" || digest != "sha256:def" {
		t.Fatalf("splitImageRefTagDigest = %q/%q", tag, digest)
	}
	if tag, digest := splitImageRefTagDigest("registry:5000/ns/app"); tag != "registry:5000/ns/app" || digest != "" {
		t.Fatalf("splitImageRefTagDigest registry only = %q/%q", tag, digest)
	}
}

func TestDigestAndObservationHelpers(t *testing.T) {
	t.Parallel()

	if got := firstDigestFromRepoDigests("\nrepo/app:latest@sha256:abc\nrepo/app:v1@sha256:def\n"); got != "sha256:abc" {
		t.Fatalf("firstDigestFromRepoDigests = %q", got)
	}
	if got := normalizeImageDigest(" repo/app@sha256:def "); got != "sha256:def" {
		t.Fatalf("normalizeImageDigest = %q", got)
	}
	if got := appendImageObservationError("local failed", "remote failed"); got != "local failed; remote failed" {
		t.Fatalf("appendImageObservationError = %q", got)
	}
}

func TestInspectRemoteImageDigestHandlesNoValue(t *testing.T) {
	logFile := installFakeDockerScript(t, "#!/bin/sh\nprintf '<no value>\n'\nprintf '%s\n' \"$*\" >> \"$TEST_DOCKER_LOG_FILE\"\n")

	_, err := inspectRemoteImageDigest(t.Context(), "ghcr.io/example/app:latest")
	if err == nil || !strings.Contains(err.Error(), "did not return a digest") {
		t.Fatalf("expected missing digest error, got %v", err)
	}
	if got := strings.TrimSpace(readAgentTestFile(t, logFile)); got != "buildx imagetools inspect --format {{.Manifest.Digest}} ghcr.io/example/app:latest" {
		t.Fatalf("docker args = %q", got)
	}
}

func TestLoadComposeConfigOutputAndCollectImageObservations(t *testing.T) {
	serviceDir := t.TempDir()
	logFile := installFakeDockerScript(t, "#!/bin/sh\nprintf '%s\n' \"$*\" >> \"$TEST_DOCKER_LOG_FILE\"\ncase \"$*\" in\n  *' config --format json')\n    printf '{\"services\":{\"api\":{\"image\":\"ghcr.io/example/api:1.0.0\"},\"worker\":{\"image\":\"ghcr.io/example/worker:2.0.0\"},\"empty\":{\"image\":\"\"}}}'\n    ;;\n  *'image inspect'*'ghcr.io/example/api:1.0.0')\n    printf 'ghcr.io/example/api:1.0.0@sha256:api\\n'\n    ;;\n  *'image inspect'*'ghcr.io/example/worker:2.0.0')\n    printf 'ghcr.io/example/worker:2.0.0@sha256:worker\\n'\n    ;;\n  *)\n    printf 'unexpected args: %s\\n' \"$*\" >&2\n    exit 2\n    ;;\nesac\n")

	compose := composeCommandConfig{ProjectName: "demo", Files: []string{"compose.yaml"}}
	config, err := loadComposeConfigOutput(t.Context(), serviceDir, compose)
	if err != nil {
		t.Fatalf("loadComposeConfigOutput returned error: %v", err)
	}
	if len(config.Services) != 3 || config.Services["api"].Image != "ghcr.io/example/api:1.0.0" {
		t.Fatalf("unexpected compose config: %+v", config)
	}

	observations, err := collectServiceImageObservations(t.Context(), serviceDir, compose, false)
	if err != nil {
		t.Fatalf("collectServiceImageObservations returned error: %v", err)
	}
	if len(observations) != 2 {
		t.Fatalf("expected 2 observations, got %+v", observations)
	}
	if observations[0].ComposeService != "api" || observations[0].LocalDigest != "sha256:api" || !observations[0].LocalObserved {
		t.Fatalf("unexpected api observation: %+v", observations[0])
	}
	if observations[1].ComposeService != "worker" || observations[1].LocalDigest != "sha256:worker" || !observations[1].LocalObserved {
		t.Fatalf("unexpected worker observation: %+v", observations[1])
	}
	if got := readAgentTestFile(t, logFile); strings.Count(got, "compose --project-name demo -f compose.yaml config --format json") != 2 {
		t.Fatalf("expected compose config to run twice, log:\n%s", got)
	}
}

func TestInspectLocalImageDigestReturnsEmptyWhenNoRepoDigest(t *testing.T) {
	installFakeDockerScript(t, "#!/bin/sh\nprintf '\n'\n")

	digest, err := inspectLocalImageDigest(t.Context(), "ghcr.io/example/app:latest")
	if err != nil {
		t.Fatalf("inspectLocalImageDigest returned error: %v", err)
	}
	if digest != "" {
		t.Fatalf("digest = %q", digest)
	}
}

func TestInspectRemoteImageDigestNormalizesDigest(t *testing.T) {
	installFakeDockerScript(t, "#!/bin/sh\nprintf 'ghcr.io/example/app:latest@sha256:remote\n'\n")

	digest, err := inspectRemoteImageDigest(t.Context(), "ghcr.io/example/app:latest")
	if err != nil {
		t.Fatalf("inspectRemoteImageDigest returned error: %v", err)
	}
	if digest != "sha256:remote" {
		t.Fatalf("digest = %q", digest)
	}
}

func TestLoadComposeConfigOutputReportsInvalidJSON(t *testing.T) {
	installFakeDockerScript(t, "#!/bin/sh\nprintf 'not-json'\n")

	_, err := loadComposeConfigOutput(t.Context(), t.TempDir(), composeCommandConfig{ProjectName: "demo"})
	if err == nil || !strings.Contains(err.Error(), "decode docker compose config json") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestCollectServiceImageObservationsKeepsLocalDigestError(t *testing.T) {
	installFakeDockerScript(t, `#!/bin/sh
case "$*" in
  *' config --format json') printf '{"services":{"api":{"image":"ghcr.io/example/api:1.0.0"}}}' ;;
  *'image inspect'*) printf 'inspect failed
' >&2; exit 7 ;;
esac
`)

	observations, err := collectServiceImageObservations(t.Context(), t.TempDir(), composeCommandConfig{ProjectName: "demo"}, false)
	if err != nil {
		t.Fatalf("collectServiceImageObservations returned error: %v", err)
	}
	if len(observations) != 1 || !strings.Contains(observations[0].ErrorSummary, "local digest") {
		t.Fatalf("expected local digest error observation, got %+v", observations)
	}
}
