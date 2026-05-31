package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSplitRegistryRepository(t *testing.T) {
	t.Parallel()

	tests := []struct {
		imageRef   string
		registry   string
		repository string
	}{
		{imageRef: "alpine", registry: "registry-1.docker.io", repository: "library/alpine"},
		{imageRef: "library/alpine", registry: "registry-1.docker.io", repository: "library/alpine"},
		{imageRef: "ghcr.io/example/app", registry: "ghcr.io", repository: "example/app"},
		{imageRef: "localhost:5000/example/app", registry: "localhost:5000", repository: "example/app"},
	}
	for _, tt := range tests {
		t.Run(tt.imageRef, func(t *testing.T) {
			t.Parallel()
			registry, repository := splitRegistryRepository(tt.imageRef)
			if registry != tt.registry || repository != tt.repository {
				t.Fatalf("splitRegistryRepository = %q/%q, want %q/%q", registry, repository, tt.registry, tt.repository)
			}
		})
	}
}

func TestParseAuthChallenge(t *testing.T) {
	t.Parallel()

	params := parseAuthChallenge(`realm="https://auth.example/token",service="registry.example",scope="repository:app:pull"`)
	if params["realm"] != "https://auth.example/token" || params["service"] != "registry.example" || params["scope"] != "repository:app:pull" {
		t.Fatalf("params = %+v", params)
	}
}

func TestNextRegistryPageURL(t *testing.T) {
	t.Parallel()

	requestURL, err := url.Parse("https://registry.example/v2/app/tags/list?n=100")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	next := nextRegistryPageURL(requestURL, []string{`</v2/app/tags/list?n=100&last=v1>; rel="next"`})
	if next != "https://registry.example/v2/app/tags/list?n=100&last=v1" {
		t.Fatalf("next = %q", next)
	}
}

func TestRegistryRequestRetriesWithBearerToken(t *testing.T) {
	t.Parallel()

	var authed bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/app/tags/list":
			if r.Header.Get("Authorization") == "Bearer registry-token" {
				authed = true
				_ = json.NewEncoder(w).Encode(map[string][]string{"tags": {"v1"}})
				return
			}
			w.Header().Set("WWW-Authenticate", `Bearer realm="`+serverURL(t, r)+`/token",service="registry.example",scope="repository:app:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
		case "/token":
			if r.URL.Query().Get("service") != "registry.example" || r.URL.Query().Get("scope") != "repository:app:pull" {
				t.Fatalf("unexpected token query: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(registryAuthToken{Token: "registry-token"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	response, _, err := registryRequest(t.Context(), server.Client(), http.MethodGet, server.URL+"/v2/app/tags/list", nil)
	if err != nil {
		t.Fatalf("registryRequest returned error: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if !authed || response.StatusCode != http.StatusOK {
		t.Fatalf("expected authed 200 response, authed=%v status=%s", authed, response.Status)
	}
}

func TestRegistryAuthRequestIgnoresNonBearerChallenge(t *testing.T) {
	t.Parallel()

	token, err := registryAuthRequest(t.Context(), http.DefaultClient, "Basic realm=example")
	if err != nil || token != nil {
		t.Fatalf("token/err = %+v/%v", token, err)
	}
	_, err = registryAuthRequest(t.Context(), http.DefaultClient, "Bearer service=registry.example")
	if err == nil || !strings.Contains(err.Error(), "missing realm") {
		t.Fatalf("expected missing realm error, got %v", err)
	}
}

func serverURL(t *testing.T, r *http.Request) string {
	t.Helper()

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
