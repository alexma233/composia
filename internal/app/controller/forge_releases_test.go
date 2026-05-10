package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
)

func TestCollectForgeImageCandidatesUsesGitHubAutoRepoURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/example/api/releases" {
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
		if request.URL.Query().Get("per_page") != "100" {
			t.Fatalf("unexpected query %q", request.URL.RawQuery)
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("unexpected auth header %q", request.Header.Get("Authorization"))
		}
		_, _ = writer.Write([]byte(`[{"tag_name":"v1.2.3"},{"tag_name":"v1.3.0"}]`))
	}))
	defer server.Close()

	auto := true
	service := repo.Service{Meta: repo.ServiceMeta{Update: &repo.UpdateConfig{Images: map[string]repo.ImageUpdateConfig{
		"api": {
			Image:     "ghcr.io/example/api",
			Current:   repo.ImageUpdateCurrent{Env: &repo.ImageUpdateCurrentEnv{File: ".env", Key: "API_VERSION"}},
			Discovery: repo.ImageUpdateDiscovery{Auto: &auto, RepoURL: "https://github.com/example/api"},
			Filter:    &repo.ImageUpdateFilter{Type: "semver"},
		},
	}}}}
	cfg := &config.ControllerConfig{Updates: &config.ControllerUpdatesConfig{ForgeAuth: &config.ControllerUpdatesForgeAuth{GitHub: config.ForgeAuthConfigs{{URL: "https://github.com", APIURL: server.URL, Token: "test-token"}}}}}

	candidates, sourceCandidates, err := collectForgeImageCandidates(context.Background(), cfg, service, []string{"api"})
	if err != nil {
		t.Fatalf("collectForgeImageCandidates() error = %v", err)
	}
	got := candidates["api"]
	if len(got) != 2 || got[0] != "v1.2.3" || got[1] != "v1.3.0" {
		t.Fatalf("unexpected candidates %+v", candidates)
	}
	if len(sourceCandidates) != 0 {
		t.Fatalf("unexpected source candidates %+v", sourceCandidates)
	}
}

func TestCollectForgeImageCandidatesSkipsAutoWithoutRepoHint(t *testing.T) {
	t.Parallel()

	auto := true
	service := repo.Service{Meta: repo.ServiceMeta{Update: &repo.UpdateConfig{Images: map[string]repo.ImageUpdateConfig{
		"api": {
			Image:     "ghcr.io/example/api",
			Current:   repo.ImageUpdateCurrent{Env: &repo.ImageUpdateCurrentEnv{File: ".env", Key: "API_VERSION"}},
			Discovery: repo.ImageUpdateDiscovery{Auto: &auto},
			Filter:    &repo.ImageUpdateFilter{Type: "semver"},
		},
	}}}}

	candidates, sourceCandidates, err := collectForgeImageCandidates(context.Background(), &config.ControllerConfig{}, service, []string{"api"})
	if err != nil {
		t.Fatalf("collectForgeImageCandidates() error = %v", err)
	}
	if len(candidates) != 0 || len(sourceCandidates) != 0 {
		t.Fatalf("expected no candidates, got %+v %+v", candidates, sourceCandidates)
	}
}

func TestForgeSourceFromRepoURLDetectsGitLab(t *testing.T) {
	t.Parallel()

	source, ok, err := forgeSourceFromRepoURL("https://gitlab.com/group/subgroup/project")
	if err != nil {
		t.Fatalf("forgeSourceFromRepoURL() error = %v", err)
	}
	if !ok || source.Type != "gitlab" || source.Project != "group/subgroup/project" {
		t.Fatalf("unexpected source %+v ok=%v", source, ok)
	}
}

func TestForgeSourceFromRepoURLDetectsCodebergAsForgejo(t *testing.T) {
	t.Parallel()

	source, ok, err := forgeSourceFromRepoURL("https://codeberg.org/owner/repo")
	if err != nil {
		t.Fatalf("forgeSourceFromRepoURL() error = %v", err)
	}
	if !ok || source.Type != "forgejo" || source.Repo != "owner/repo" || source.APIURL != "https://codeberg.org/api/v1" {
		t.Fatalf("unexpected source %+v ok=%v", source, ok)
	}
}
