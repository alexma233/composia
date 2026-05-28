package agent

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
)

func TestParseSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  uint64
		ok    bool
	}{
		{name: "bytes", input: "512B", want: 512, ok: true},
		{name: "decimal kilobytes", input: "1.5kB", want: 1500, ok: true},
		{name: "decimal gigabytes", input: "4.8GB", want: 4800000000, ok: true},
		{name: "binary gibibytes", input: "1.5GiB", want: 1610612736, ok: true},
		{name: "decimal terabytes", input: "2.8TB", want: 2800000000000, ok: true},
		{name: "comma decimal", input: "1,5MB", want: 1500000, ok: true},
		{name: "invalid", input: "oops", want: 0, ok: false},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseSize(tc.input)
			if ok != tc.ok {
				t.Fatalf("parseSize(%q) ok = %v, want %v", tc.input, ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("parseSize(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseDockerSystemDFVolumeSize(t *testing.T) {
	t.Parallel()

	output := []byte("Images\t4.8GB\nContainers\t20kB\nLocal Volumes\t1.5MB\nBuild Cache\t512B\n")

	got := parseDockerSystemDFVolumeSize(output)
	if got != 1500000 {
		t.Fatalf("parseDockerSystemDFVolumeSize() = %d, want %d", got, uint64(1500000))
	}
}

func TestDecodeTaskParamsReturnsJSONError(t *testing.T) {
	t.Parallel()

	if _, err := decodeTaskParams("{"); err == nil {
		t.Fatalf("expected invalid task params JSON to fail")
	}
}

func TestParseRusticMaintenanceParamsReturnsJSONError(t *testing.T) {
	t.Parallel()

	if _, err := parseRusticMaintenanceParams(&agentv1.AgentTask{ParamsJson: "{"}); err == nil {
		t.Fatalf("expected invalid rustic maintenance params JSON to fail")
	}
}

func TestEnsureAgentDirsCreatesDataProtectDirOnStartup(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	cfg := &config.AgentConfig{
		RepoDir:  filepath.Join(rootDir, "repo"),
		StateDir: filepath.Join(rootDir, "state"),
	}

	if err := ensureAgentDirs(cfg); err != nil {
		t.Fatalf("ensure agent dirs: %v", err)
	}

	for _, dir := range []string{
		cfg.StateDir,
		dataProtectStageRoot(cfg.StateDir),
		cfg.RepoDir,
		cfg.CaddyGeneratedDir(),
	} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %q: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %q to be a directory", dir)
		}
	}
}

func TestCandidateImageTagsReturnsNewestCandidatesFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current string
		tags    []string
		filter  repo.ImageUpdateFilter
		want    []string
	}{
		{
			name:    "semver",
			current: "1.2.3",
			tags:    []string{"1.2.4", "1.3.0", "2.0.0", "1.2.5"},
			filter:  repo.ImageUpdateFilter{Type: "semver", Allow: []string{"patch", "minor"}},
			want:    []string{"1.3.0", "1.2.5", "1.2.4"},
		},
		{
			name:    "date",
			current: "2026-05-01",
			tags:    []string{"2026-05-03", "2026-05-02", "2026-04-30"},
			filter:  repo.ImageUpdateFilter{Type: "date", Format: "2006-01-02"},
			want:    []string{"2026-05-03", "2026-05-02"},
		},
		{
			name:    "regex numeric",
			current: "build-10",
			tags:    []string{"build-11", "build-13", "build-12", "build-9"},
			filter:  repo.ImageUpdateFilter{Type: "regex", Pattern: `^build-(\d+)$`, Order: "numeric"},
			want:    []string{"build-13", "build-12", "build-11"},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := candidateImageTags(tc.current, tc.tags, &tc.filter)
			if len(got) != len(tc.want) {
				t.Fatalf("candidateImageTags() = %+v, want %+v", got, tc.want)
			}
			for index := range tc.want {
				if got[index] != tc.want[index] {
					t.Fatalf("candidateImageTags() = %+v, want %+v", got, tc.want)
				}
			}
		})
	}
}

func TestProbeSemverImageTagWithExistsPrefersHighestAllowedCandidate(t *testing.T) {
	t.Parallel()

	exists := map[string]struct{}{
		"1.2.4": {},
		"1.2.5": {},
		"1.3.0": {},
		"1.3.1": {},
		"2.0.0": {},
	}

	got, found, err := probeSemverImageTagWithExists(context.Background(), "ghcr.io/example/app", "1.2.3", &repo.ImageUpdateFilter{Type: "semver", Allow: []string{"patch", "minor", "major"}}, func(_ context.Context, _ string, tag string) (bool, error) {
		_, ok := exists[tag]
		return ok, nil
	})
	if err != nil {
		t.Fatalf("probeSemverImageTagWithExists() error = %v", err)
	}
	if !found {
		t.Fatalf("probeSemverImageTagWithExists() found = false, want true")
	}
	if got != "2.0.0" {
		t.Fatalf("probeSemverImageTagWithExists() = %q, want %q", got, "2.0.0")
	}
}

func TestProbeSemverImageTagWithExistsPreservesVPrefix(t *testing.T) {
	t.Parallel()

	got, found, err := probeSemverImageTagWithExists(context.Background(), "ghcr.io/example/app", "v1.2.3", &repo.ImageUpdateFilter{Type: "semver", Allow: []string{"patch"}}, func(_ context.Context, _ string, tag string) (bool, error) {
		return tag == "v1.2.4", nil
	})
	if err != nil {
		t.Fatalf("probeSemverImageTagWithExists() error = %v", err)
	}
	if !found {
		t.Fatalf("probeSemverImageTagWithExists() found = false, want true")
	}
	if got != "v1.2.4" {
		t.Fatalf("probeSemverImageTagWithExists() = %q, want %q", got, "v1.2.4")
	}
}

func TestProbeSemverImageTagWithExistsReturnsNotFoundWhenNextLineMissing(t *testing.T) {
	t.Parallel()

	got, found, err := probeSemverImageTagWithExists(context.Background(), "ghcr.io/example/app", "1.2.3", &repo.ImageUpdateFilter{Type: "semver", Allow: []string{"minor"}}, func(_ context.Context, _ string, tag string) (bool, error) {
		return tag == "1.4.0", nil
	})
	if err != nil {
		t.Fatalf("probeSemverImageTagWithExists() error = %v", err)
	}
	if found {
		t.Fatalf("probeSemverImageTagWithExists() found = true with %q, want false", got)
	}
}

func TestNextRegistryPageURLResolvesRelativeLink(t *testing.T) {
	t.Parallel()

	requestURL, err := url.Parse("https://registry-1.docker.io/v2/library/nginx/tags/list?n=100")
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	got := nextRegistryPageURL(requestURL, []string{`</v2/library/nginx/tags/list?n=100&last=1.25.0>; rel="next"`})
	if got != "https://registry-1.docker.io/v2/library/nginx/tags/list?n=100&last=1.25.0" {
		t.Fatalf("nextRegistryPageURL() = %q", got)
	}
}

func TestMergeImageUpdateTagsDeduplicatesInjectedCandidatesFirst(t *testing.T) {
	t.Parallel()

	got := mergeImageUpdateTags([]string{"v1.2.0", "v1.3.0"}, []string{"v1.3.0", "v1.4.0"})
	want := []string{"v1.2.0", "v1.3.0", "v1.4.0"}
	if len(got) != len(want) {
		t.Fatalf("mergeImageUpdateTags() = %+v, want %+v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("mergeImageUpdateTags() = %+v, want %+v", got, want)
		}
	}
}
