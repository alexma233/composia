package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const registryTagsPageSize = 100

func listRegistryTags(ctx context.Context, imageRef string) ([]string, error) {
	registry, repository := splitRegistryRepository(imageRef)
	client := &http.Client{Timeout: 30 * time.Second}
	endpoint := fmt.Sprintf("https://%s/v2/%s/tags/list?n=%d", registry, repository, registryTagsPageSize)
	tags := make([]string, 0, registryTagsPageSize)
	for endpoint != "" {
		response, requestURL, err := registryRequest(ctx, client, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("list registry tags for %q: %w", imageRef, err)
		}
		func() {
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				err = fmt.Errorf("list registry tags for %q returned %s", imageRef, response.Status)
				return
			}
			var payload struct {
				Tags []string `json:"tags"`
			}
			if decodeErr := json.NewDecoder(response.Body).Decode(&payload); decodeErr != nil {
				err = fmt.Errorf("decode registry tags for %q: %w", imageRef, decodeErr)
				return
			}
			tags = append(tags, payload.Tags...)
			endpoint = nextRegistryPageURL(requestURL, response.Header.Values("Link"))
		}()
		if err != nil {
			return nil, err
		}
	}
	return tags, nil
}

func registryManifestExists(ctx context.Context, imageRef, tag string) (bool, error) {
	registry, repository := splitRegistryRepository(imageRef)
	endpoint := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, repository, url.PathEscape(strings.TrimSpace(tag)))
	headers := map[string]string{
		"Accept": strings.Join([]string{
			"application/vnd.oci.image.index.v1+json",
			"application/vnd.oci.image.manifest.v1+json",
			"application/vnd.docker.distribution.manifest.list.v2+json",
			"application/vnd.docker.distribution.manifest.v2+json",
		}, ", "),
	}
	client := &http.Client{Timeout: 30 * time.Second}
	method := http.MethodHead
	response, _, err := registryRequest(ctx, client, method, endpoint, headers)
	if err != nil {
		return false, fmt.Errorf("probe registry manifest for %q tag %q: %w", imageRef, tag, err)
	}
	if response.StatusCode == http.StatusMethodNotAllowed {
		_ = response.Body.Close()
		method = http.MethodGet
		response, _, err = registryRequest(ctx, client, http.MethodGet, endpoint, headers)
		if err != nil {
			return false, fmt.Errorf("probe registry manifest for %q tag %q: %w", imageRef, tag, err)
		}
	}
	defer func() { _ = response.Body.Close() }()
	switch response.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("registry manifest probe for %q tag %q with %s returned %s", imageRef, tag, method, response.Status)
	}
}

func splitRegistryRepository(imageRef string) (string, string) {
	parts := strings.Split(imageRef, "/")
	if len(parts) == 1 || (!strings.Contains(parts[0], ".") && !strings.Contains(parts[0], ":") && parts[0] != "localhost") {
		repository := imageRef
		if !strings.Contains(repository, "/") {
			repository = "library/" + repository
		}
		return "registry-1.docker.io", repository
	}
	return parts[0], strings.Join(parts[1:], "/")
}

type registryAuthToken struct {
	Token string `json:"token"`
}

func registryAuthRequest(ctx context.Context, client *http.Client, challenge string) (*registryAuthToken, error) {
	if !strings.HasPrefix(strings.ToLower(challenge), "bearer ") {
		return nil, nil //nolint:nilnil
	}
	params := parseAuthChallenge(challenge[len("Bearer "):])
	realm := params["realm"]
	if realm == "" {
		return nil, errors.New("registry auth challenge is missing realm")
	}
	values := url.Values{}
	for _, key := range []string{"service", "scope"} {
		if params[key] != "" {
			values.Set(key, params[key])
		}
	}
	requestURL := realm
	if encoded := values.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request registry auth token: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("request registry auth token returned %s", response.Status)
	}
	var token registryAuthToken
	if err := json.NewDecoder(response.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decode registry auth token: %w", err)
	}
	return &token, nil
}

func registryRequest(ctx context.Context, client *http.Client, method, endpoint string, headers map[string]string) (*http.Response, *url.URL, error) {
	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, nil, err
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, nil, err
	}
	if response.StatusCode != http.StatusUnauthorized {
		return response, requestURL, nil
	}
	authRequest, err := registryAuthRequest(ctx, client, response.Header.Get("WWW-Authenticate"))
	if err != nil {
		_ = response.Body.Close()
		return nil, nil, err
	}
	if authRequest == nil {
		return response, requestURL, nil
	}
	if err := response.Body.Close(); err != nil {
		return nil, nil, err
	}
	request, err = http.NewRequestWithContext(ctx, method, requestURL.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	request.Header.Set("Authorization", "Bearer "+authRequest.Token)
	response, err = client.Do(request)
	if err != nil {
		return nil, nil, err
	}
	return response, requestURL, nil
}

func nextRegistryPageURL(requestURL *url.URL, linkHeaders []string) string {
	for _, header := range linkHeaders {
		for _, part := range strings.Split(header, ",") {
			part = strings.TrimSpace(part)
			if part == "" || !strings.Contains(part, `rel="next"`) {
				continue
			}
			start := strings.Index(part, "<")
			end := strings.Index(part, ">")
			if start < 0 || end <= start+1 {
				continue
			}
			next := strings.TrimSpace(part[start+1 : end])
			nextURL, err := requestURL.Parse(next)
			if err != nil {
				return ""
			}
			return nextURL.String()
		}
	}
	return ""
}

func parseAuthChallenge(value string) map[string]string {
	params := make(map[string]string)
	for _, part := range strings.Split(value, ",") {
		key, rawValue, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		params[strings.ToLower(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(rawValue), `"`)
	}
	return params
}
