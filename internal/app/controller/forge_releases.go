package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"github.com/tomnomnom/linkheader"
)

func collectForgeImageCandidates(ctx context.Context, cfg *config.ControllerConfig, service repo.Service, imageNames []string) (map[string][]string, map[string]map[string][]string, error) {
	if service.Meta.Update == nil || len(service.Meta.Update.Images) == 0 {
		return nil, nil, nil
	}
	if len(imageNames) == 0 {
		imageNames = make([]string, 0, len(service.Meta.Update.Images))
		for imageName := range service.Meta.Update.Images {
			imageNames = append(imageNames, imageName)
		}
	}
	mergedResult := make(map[string][]string)
	sourceResult := make(map[string]map[string][]string)
	for _, imageName := range imageNames {
		image, ok := service.Meta.Update.Images[imageName]
		if !ok {
			continue
		}
		discovery := repo.ResolveImageUpdateDiscovery(image.Discovery, service.Meta.Update.DiscoverySources)
		merged, bySource, err := collectForgeCandidatesForDiscovery(ctx, cfg, discovery)
		if err != nil {
			return nil, nil, fmt.Errorf("collect forge candidates for image %q: %w", imageName, err)
		}
		if len(merged) > 0 {
			mergedResult[imageName] = merged
		}
		if len(bySource) > 0 {
			sourceResult[imageName] = bySource
		}
	}
	if len(mergedResult) == 0 {
		mergedResult = nil
	}
	if len(sourceResult) == 0 {
		sourceResult = nil
	}
	return mergedResult, sourceResult, nil
}

func collectForgeCandidatesForDiscovery(ctx context.Context, cfg *config.ControllerConfig, discovery repo.ImageUpdateDiscovery) ([]string, map[string][]string, error) {
	includePrerelease := discovery.IncludePrerelease != nil && *discovery.IncludePrerelease
	if len(discovery.Sources) == 1 && strings.TrimSpace(discovery.Sources[0].Type) == imageUpdateDiscoveryAuto {
		autoSource := discovery.Sources[0]
		if strings.TrimSpace(autoSource.RepoURL) == "" {
			return nil, nil, nil
		}
		source, ok, err := forgeSourceFromRepoURL(autoSource.RepoURL)
		if err != nil || !ok {
			return nil, nil, err
		}
		candidates, err := fetchForgeReleaseTags(ctx, cfg, source, includePrerelease)
		return candidates, nil, err
	}
	if discovery.Combine == imageUpdateDiscoveryMerge {
		candidates, err := mergeForgeCandidateGroups(ctx, cfg, discovery.Sources, includePrerelease)
		return candidates, nil, err
	}
	bySource := make(map[string][]string)
	for index, source := range discovery.Sources {
		if !isForgeDiscoverySource(source.Type) {
			continue
		}
		candidates, err := fetchForgeReleaseTags(ctx, cfg, source, includePrerelease)
		if err != nil {
			return nil, nil, err
		}
		if len(candidates) > 0 {
			bySource[strconv.Itoa(index)] = candidates
		}
	}
	return nil, bySource, nil
}

func forgeSourceFromRepoURL(rawURL string) (repo.ImageUpdateDiscoverySource, bool, error) {
	repoURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return repo.ImageUpdateDiscoverySource{}, false, fmt.Errorf("parse repo_url %q: %w", rawURL, err)
	}
	host := strings.ToLower(repoURL.Hostname())
	parts := make([]string, 0)
	for _, part := range strings.Split(strings.Trim(repoURL.Path, "/"), "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) < 2 {
		return repo.ImageUpdateDiscoverySource{}, false, fmt.Errorf("repo_url %q must include owner/repo path", rawURL)
	}
	repoPath := strings.Join(parts[:2], "/")
	fullProject := strings.Join(parts, "/")
	switch host {
	case "github.com":
		return repo.ImageUpdateDiscoverySource{Type: imageUpdateDiscoveryGitHub, Repo: repoPath, RepoURL: rawURL}, true, nil
	case "gitlab.com":
		return repo.ImageUpdateDiscoverySource{Type: imageUpdateDiscoveryGitLab, Project: fullProject, RepoURL: rawURL}, true, nil
	case "codeberg.org":
		return repo.ImageUpdateDiscoverySource{Type: imageUpdateDiscoveryForgejo, Repo: repoPath, RepoURL: rawURL, APIURL: "https://codeberg.org/api/v1"}, true, nil
	default:
		return repo.ImageUpdateDiscoverySource{}, false, nil
	}
}

func mergeForgeCandidateGroups(ctx context.Context, cfg *config.ControllerConfig, sources []repo.ImageUpdateDiscoverySource, includePrerelease bool) ([]string, error) {
	seen := make(map[string]struct{})
	merged := make([]string, 0)
	for _, source := range sources {
		if !isForgeDiscoverySource(source.Type) {
			continue
		}
		candidates, err := fetchForgeReleaseTags(ctx, cfg, source, includePrerelease)
		if err != nil {
			return nil, err
		}
		for _, candidate := range candidates {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				continue
			}
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			merged = append(merged, candidate)
		}
	}
	return merged, nil
}

func isForgeDiscoverySource(sourceType string) bool {
	switch sourceType {
	case imageUpdateDiscoveryGitHub, imageUpdateDiscoveryGitLab, imageUpdateDiscoveryForgejo:
		return true
	default:
		return false
	}
}

func fetchForgeReleaseTags(ctx context.Context, cfg *config.ControllerConfig, source repo.ImageUpdateDiscoverySource, includePrerelease bool) ([]string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	endpoint, headers, err := forgeReleaseRequest(source, cfg)
	if err != nil {
		return nil, err
	}
	tags := make([]string, 0)
	pageCount := 0
	for endpoint != "" {
		pageCount++
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		for key, value := range headers {
			request.Header.Set(key, value)
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("request %s releases: %w", source.Type, err)
		}
		func() {
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				err = fmt.Errorf("request %s releases returned %s", source.Type, response.Status)
				return
			}
			var releases []struct {
				TagName    string `json:"tag_name"`
				Prerelease bool   `json:"prerelease"`
			}
			if decodeErr := json.NewDecoder(response.Body).Decode(&releases); decodeErr != nil {
				err = fmt.Errorf("decode %s releases: %w", source.Type, decodeErr)
				return
			}
			for _, release := range releases {
				tag := strings.TrimSpace(release.TagName)
				if tag == "" {
					continue
				}
				if !includePrerelease && release.Prerelease {
					continue
				}
				tags = append(tags, tag)
			}
			endpoint = forgeNextPageURL(response.Header.Values("Link"))
		}()
		if err != nil {
			return nil, err
		}
		if len(tags) > 0 || pageCount >= 3 {
			break
		}
	}
	return tags, nil
}

func forgeNextPageURL(linkHeaders []string) string {
	for _, link := range linkheader.ParseMultiple(linkHeaders).FilterByRel("next") {
		return strings.TrimSpace(link.URL)
	}
	return ""
}

func forgeReleaseRequest(source repo.ImageUpdateDiscoverySource, cfg *config.ControllerConfig) (string, map[string]string, error) {
	headers := map[string]string{"Accept": "application/json"}
	updates := (*config.ControllerUpdatesConfig)(nil)
	if cfg != nil {
		updates = cfg.Updates
	}
	switch source.Type {
	case imageUpdateDiscoveryGitHub:
		repoName := strings.TrimSpace(source.Repo)
		if repoName == "" {
			return "", nil, errors.New("github release discovery requires repo")
		}
		auth := forgeAuth(updates, imageUpdateDiscoveryGitHub, source.RepoURL)
		apiURL := strings.TrimRight(auth.APIURL, "/")
		if apiURL == "" {
			apiURL = "https://api.github.com"
		}
		if auth.Token != "" {
			headers["Authorization"] = "Bearer " + auth.Token
		}
		return apiURL + "/repos/" + strings.Trim(repoName, "/") + "/releases?per_page=100", headers, nil
	case imageUpdateDiscoveryGitLab:
		project := strings.TrimSpace(source.Project)
		if project == "" {
			return "", nil, errors.New("gitlab release discovery requires project")
		}
		auth := forgeAuth(updates, imageUpdateDiscoveryGitLab, source.RepoURL)
		apiURL := strings.TrimRight(auth.APIURL, "/")
		if apiURL == "" {
			apiURL = "https://gitlab.com/api/v4"
		}
		if auth.Token != "" {
			headers["PRIVATE-TOKEN"] = auth.Token
		}
		return apiURL + "/projects/" + url.PathEscape(project) + "/releases?per_page=100", headers, nil
	case imageUpdateDiscoveryForgejo:
		repoName := strings.TrimSpace(source.Repo)
		if repoName == "" {
			return "", nil, errors.New("forgejo release discovery requires repo")
		}
		auth := forgeAuth(updates, imageUpdateDiscoveryForgejo, source.RepoURL)
		apiURL := strings.TrimRight(source.APIURL, "/")
		if apiURL == "" {
			apiURL = strings.TrimRight(auth.APIURL, "/")
		}
		if apiURL == "" {
			return "", nil, errors.New("controller.updates.forge_auth.forgejo.api_url is required")
		}
		if auth.Token != "" {
			headers["Authorization"] = "token " + auth.Token
		}
		return apiURL + "/repos/" + strings.Trim(repoName, "/") + "/releases?limit=100", headers, nil
	default:
		return "", nil, fmt.Errorf("unsupported forge release discovery %q", source.Type)
	}
}

func forgeAuth(updates *config.ControllerUpdatesConfig, sourceType, repoURL string) config.ForgeAuthConfig {
	if updates == nil || updates.ForgeAuth == nil {
		return config.ForgeAuthConfig{}
	}
	var auths config.ForgeAuthConfigs
	switch sourceType {
	case imageUpdateDiscoveryGitHub:
		auths = updates.ForgeAuth.GitHub
	case imageUpdateDiscoveryGitLab:
		auths = updates.ForgeAuth.GitLab
	case imageUpdateDiscoveryForgejo:
		auths = updates.ForgeAuth.Forgejo
	}
	if len(auths) == 0 {
		return config.ForgeAuthConfig{}
	}
	if repoURL != "" {
		if repoHost := normalizedURLHost(repoURL); repoHost != "" {
			for _, auth := range auths {
				if auth.URL != "" && normalizedURLHost(auth.URL) == repoHost {
					return auth
				}
				if auth.APIURL != "" && normalizedURLHost(auth.APIURL) == repoHost {
					return auth
				}
			}
		}
	}
	for _, auth := range auths {
		if auth.URL == "" && auth.APIURL == "" {
			return auth
		}
	}
	return config.ForgeAuthConfig{}
}

func normalizedURLHost(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}
