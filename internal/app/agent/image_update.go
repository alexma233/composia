package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"
)

type serviceImageObservation struct {
	ComposeService string
	ImageRef       string
	LocalDigest    string
	RemoteDigest   string
	LocalObserved  bool
	RemoteObserved bool
	ErrorSummary   string
}

func reportServiceImageStatesBestEffort(ctx context.Context, client agentv1connect.AgentReportServiceClient, pulledTask *agentv1.AgentTask, serviceDir string, includeRemote bool, logUploader *taskLogUploader) {
	if err := reportServiceImageStates(ctx, client, pulledTask, serviceDir, includeRemote, logUploader); err != nil {
		if logErr := uploadTaskLog(ctx, logUploader, fmt.Sprintf("warning: could not report service image states: %v\n", err)); logErr != nil {
			log.Printf("upload image state warning for task=%s: %v", pulledTask.GetTaskId(), logErr)
		}
	}
}

func reportServiceImageStates(ctx context.Context, client agentv1connect.AgentReportServiceClient, pulledTask *agentv1.AgentTask, serviceDir string, includeRemote bool, logUploader *taskLogUploader) error {
	compose, _, err := loadComposeCommandConfig(serviceDir, pulledTask.GetServiceName())
	if err != nil {
		return err
	}
	observations, err := collectServiceImageObservations(ctx, serviceDir, compose, includeRemote)
	if err != nil {
		return err
	}
	if len(observations) == 0 {
		return uploadTaskLog(ctx, logUploader, "no compose service images found to report\n")
	}
	images := make([]*agentv1.ServiceImageState, 0, len(observations))
	for _, observation := range observations {
		status := store.ImageCheckStatusOK
		if observation.ErrorSummary != "" {
			status = store.ImageCheckStatusError
		}
		images = append(images, &agentv1.ServiceImageState{
			ComposeService:       observation.ComposeService,
			ImageRef:             observation.ImageRef,
			LocalDigest:          observation.LocalDigest,
			RemoteDigest:         observation.RemoteDigest,
			LocalDigestObserved:  observation.LocalObserved,
			RemoteDigestObserved: observation.RemoteObserved,
			CheckStatus:          status,
			ErrorSummary:         observation.ErrorSummary,
		})
	}
	_, err = client.ReportServiceImageStates(ctx, connect.NewRequest(&agentv1.ReportServiceImageStatesRequest{
		ServiceName: pulledTask.GetServiceName(),
		NodeId:      pulledTask.GetNodeId(),
		Images:      images,
		ReportedAt:  timestamppb.Now(),
	}))
	if err != nil {
		return fmt.Errorf("report service image states: %w", err)
	}
	return uploadTaskLog(ctx, logUploader, fmt.Sprintf("reported %d service image state(s)\n", len(images)))
}

func reportServiceImageUpdateChecks(ctx context.Context, client agentv1connect.AgentReportServiceClient, pulledTask *agentv1.AgentTask, serviceDir string, serviceMeta repo.ServiceMeta, logUploader *taskLogUploader) error {
	if serviceMeta.Update == nil || len(serviceMeta.Update.Images) == 0 {
		return uploadTaskLog(ctx, logUploader, "no configured image update checks found\n")
	}
	params, err := decodeTaskParams(pulledTask.GetParamsJson())
	if err != nil {
		return err
	}
	selected := make(map[string]struct{}, len(params.ImageNames))
	for _, imageName := range params.ImageNames {
		imageName = strings.TrimSpace(imageName)
		if imageName != "" {
			selected[imageName] = struct{}{}
		}
	}
	imageNames := make([]string, 0, len(serviceMeta.Update.Images))
	for imageName := range serviceMeta.Update.Images {
		if len(selected) > 0 {
			if _, ok := selected[imageName]; !ok {
				continue
			}
		}
		imageNames = append(imageNames, imageName)
	}
	slices.Sort(imageNames)
	checks := make([]*agentv1.ServiceImageUpdateCheck, 0, len(imageNames))
	for _, imageName := range imageNames {
		image := serviceMeta.Update.Images[imageName]
		if image.Policy.Type == "semver" && len(image.Policy.Allow) == 0 && len(params.SemverAllow) > 0 {
			image.Policy.Allow = append([]string(nil), params.SemverAllow...)
		}
		checks = append(checks, collectServiceImageUpdateCheck(ctx, serviceDir, imageName, image))
	}
	if len(checks) == 0 {
		return uploadTaskLog(ctx, logUploader, "no selected image update checks found\n")
	}
	_, err = client.ReportServiceImageUpdateChecks(ctx, connect.NewRequest(&agentv1.ReportServiceImageUpdateChecksRequest{
		ServiceName: pulledTask.GetServiceName(),
		NodeId:      pulledTask.GetNodeId(),
		Checks:      checks,
		ReportedAt:  timestamppb.Now(),
	}))
	if err != nil {
		return fmt.Errorf("report service image update checks: %w", err)
	}
	return uploadTaskLog(ctx, logUploader, fmt.Sprintf("reported %d service image update check(s)\n", len(checks)))
}

func collectServiceImageUpdateCheck(ctx context.Context, serviceDir, imageName string, image repo.ImageUpdateConfig) *agentv1.ServiceImageUpdateCheck {
	check := &agentv1.ServiceImageUpdateCheck{ImageName: imageName, ImageRef: image.Image, PolicyType: image.Policy.Type, CheckStatus: store.ImageCheckStatusOK}
	currentValue, currentTag, currentDigest, err := currentImageUpdateValue(serviceDir, image)
	if err != nil {
		check.CheckStatus = store.ImageCheckStatusError
		check.ErrorSummary = err.Error()
		return check
	}
	check.CurrentValue = currentValue
	check.CurrentTag = currentTag
	check.CurrentDigest = currentDigest

	if image.Policy.Type == "mutable_digest" {
		remoteDigest, err := inspectRemoteImageDigest(ctx, image.Image+":"+currentTag)
		if err != nil {
			check.CheckStatus = store.ImageCheckStatusError
			check.ErrorSummary = err.Error()
			return check
		}
		if currentDigest == "" {
			localDigest, err := inspectLocalImageDigest(ctx, image.Image+":"+currentTag)
			if err == nil {
				currentDigest = localDigest
				check.CurrentDigest = localDigest
			}
		}
		check.CandidateTag = currentTag
		check.CandidateDigest = remoteDigest
		check.UpdateAvailable = remoteDigest != "" && currentDigest != "" && remoteDigest != currentDigest
		return check
	}

	tags, err := listRegistryTags(ctx, image.Image)
	if err != nil {
		check.CheckStatus = store.ImageCheckStatusError
		check.ErrorSummary = err.Error()
		return check
	}
	candidates := candidateImageTags(currentTag, tags, image.Policy)
	check.CandidateTags = candidates
	if len(candidates) == 0 {
		return check
	}
	check.CandidateTag = candidates[0]
	digest, err := inspectRemoteImageDigest(ctx, image.Image+":"+check.CandidateTag)
	if err != nil {
		check.CheckStatus = store.ImageCheckStatusError
		check.ErrorSummary = err.Error()
		return check
	}
	check.CandidateDigest = digest
	check.UpdateAvailable = true
	return check
}

func currentImageUpdateValue(serviceDir string, image repo.ImageUpdateConfig) (string, string, string, error) {
	if image.Source.Tag != "" {
		return image.Source.Tag, image.Source.Tag, "", nil
	}
	if image.Source.File == "" {
		return "", "", "", fmt.Errorf("image update source file is required")
	}
	path := filepath.Join(serviceDir, filepath.FromSlash(image.Source.File))
	content, err := os.ReadFile(path)
	if err != nil {
		return "", "", "", fmt.Errorf("read image update source %q: %w", image.Source.File, err)
	}
	if image.Source.Key != "" {
		value, err := envFileValue(string(content), image.Source.Key)
		if err != nil {
			return "", "", "", err
		}
		tag, digest := splitTagDigest(value)
		return value, tag, digest, nil
	}
	if image.Source.Path != "" {
		value, err := yamlPathStringValue(content, image.Source.Path)
		if err != nil {
			return "", "", "", err
		}
		tag, digest := splitImageRefTagDigest(value)
		return value, tag, digest, nil
	}
	return "", "", "", fmt.Errorf("image update source key or path is required")
}

func envFileValue(content, key string) (string, error) {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		name, value, ok := strings.Cut(trimmed, "=")
		if ok && strings.TrimSpace(name) == key {
			return strings.Trim(strings.TrimSpace(value), `"'`), nil
		}
	}
	return "", fmt.Errorf("source key %q not found", key)
}

func yamlPathStringValue(content []byte, path string) (string, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(content, &node); err != nil {
		return "", fmt.Errorf("decode yaml source: %w", err)
	}
	current := &node
	if current.Kind == yaml.DocumentNode && len(current.Content) > 0 {
		current = current.Content[0]
	}
	for _, part := range strings.Split(path, ".") {
		if current.Kind != yaml.MappingNode {
			return "", fmt.Errorf("source path %q is not a mapping", path)
		}
		var next *yaml.Node
		for index := 0; index+1 < len(current.Content); index += 2 {
			if current.Content[index].Value == part {
				next = current.Content[index+1]
				break
			}
		}
		if next == nil {
			return "", fmt.Errorf("source path %q not found", path)
		}
		current = next
	}
	if current.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("source path %q is not scalar", path)
	}
	return strings.TrimSpace(current.Value), nil
}

func splitTagDigest(value string) (string, string) {
	value = strings.TrimSpace(value)
	tag, digest, hasDigest := strings.Cut(value, "@")
	if !hasDigest {
		return tag, ""
	}
	return tag, digest
}

func splitImageRefTagDigest(value string) (string, string) {
	value = strings.TrimSpace(value)
	if at := strings.LastIndex(value, "@"); at >= 0 {
		tag, _ := splitImageRefTagDigest(value[:at])
		return tag, value[at+1:]
	}
	if colon := strings.LastIndex(value, ":"); colon >= 0 && !strings.Contains(value[colon+1:], "/") {
		return value[colon+1:], ""
	}
	return value, ""
}

func candidateImageTags(current string, tags []string, policy repo.ImageUpdatePolicy) []string {
	current = strings.TrimSpace(current)
	candidates := make([]string, 0, len(tags))
	switch policy.Type {
	case "semver":
		currentVersion, ok := parseSimpleSemver(current)
		if !ok {
			return nil
		}
		allowed := semverAllowedUpdates(policy.Allow)
		versions := make([]simpleSemverTag, 0, len(tags))
		for _, tag := range tags {
			version, ok := parseSimpleSemver(tag)
			if !ok || !version.greaterThan(currentVersion) || !semverUpdateAllowed(currentVersion, version, allowed) {
				continue
			}
			versions = append(versions, simpleSemverTag{Tag: tag, Version: version})
		}
		slices.SortFunc(versions, func(left, right simpleSemverTag) int { return right.Version.compare(left.Version) })
		for _, version := range versions {
			candidates = append(candidates, version.Tag)
		}
	case "date":
		currentTime, err := time.Parse(policy.Format, current)
		if err != nil {
			return nil
		}
		type dateTag struct {
			Tag string
			At  time.Time
		}
		dateTags := make([]dateTag, 0, len(tags))
		for _, tag := range tags {
			parsed, err := time.Parse(policy.Format, tag)
			if err == nil && parsed.After(currentTime) {
				dateTags = append(dateTags, dateTag{Tag: tag, At: parsed})
			}
		}
		slices.SortFunc(dateTags, func(left, right dateTag) int { return right.At.Compare(left.At) })
		for _, tag := range dateTags {
			candidates = append(candidates, tag.Tag)
		}
	case "regex":
		re, err := regexp.Compile(policy.Pattern)
		if err != nil {
			return nil
		}
		currentKey, ok := regexOrderKey(re, current, policy.Order)
		if !ok {
			return nil
		}
		type regexTag struct {
			Tag string
			Key string
		}
		regexTags := make([]regexTag, 0, len(tags))
		for _, tag := range tags {
			key, ok := regexOrderKey(re, tag, policy.Order)
			if !ok || compareRegexKeys(key, currentKey, policy.Order) <= 0 {
				continue
			}
			regexTags = append(regexTags, regexTag{Tag: tag, Key: key})
		}
		slices.SortFunc(regexTags, func(left, right regexTag) int { return compareRegexKeys(right.Key, left.Key, policy.Order) })
		for _, tag := range regexTags {
			candidates = append(candidates, tag.Tag)
		}
	}
	return candidates
}

type simpleSemver struct{ Major, Minor, Patch int }
type simpleSemverTag struct {
	Tag     string
	Version simpleSemver
}

func parseSimpleSemver(value string) (simpleSemver, bool) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return simpleSemver{}, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return simpleSemver{}, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return simpleSemver{}, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return simpleSemver{}, false
	}
	return simpleSemver{Major: major, Minor: minor, Patch: patch}, true
}

func (version simpleSemver) compare(other simpleSemver) int {
	if version.Major != other.Major {
		return version.Major - other.Major
	}
	if version.Minor != other.Minor {
		return version.Minor - other.Minor
	}
	return version.Patch - other.Patch
}

func (version simpleSemver) greaterThan(other simpleSemver) bool { return version.compare(other) > 0 }

func semverAllowedUpdates(allow []string) map[string]struct{} {
	if len(allow) == 0 {
		allow = []string{"patch", "minor"}
	}
	allowed := make(map[string]struct{}, len(allow))
	for _, item := range allow {
		allowed[strings.TrimSpace(item)] = struct{}{}
	}
	return allowed
}

func semverUpdateAllowed(current, candidate simpleSemver, allowed map[string]struct{}) bool {
	updateType := "patch"
	if candidate.Major != current.Major {
		updateType = "major"
	} else if candidate.Minor != current.Minor {
		updateType = "minor"
	}
	_, ok := allowed[updateType]
	return ok
}

func regexOrderKey(re *regexp.Regexp, value, order string) (string, bool) {
	matches := re.FindStringSubmatch(value)
	if len(matches) == 0 {
		return "", false
	}
	key := matches[0]
	if len(matches) > 1 {
		key = matches[1]
	}
	if order == "numeric" {
		if _, err := strconv.ParseInt(key, 10, 64); err != nil {
			return "", false
		}
	}
	return key, true
}

func compareRegexKeys(left, right, order string) int {
	if order == "numeric" {
		leftNumber, _ := strconv.ParseInt(left, 10, 64)
		rightNumber, _ := strconv.ParseInt(right, 10, 64)
		switch {
		case leftNumber < rightNumber:
			return -1
		case leftNumber > rightNumber:
			return 1
		default:
			return 0
		}
	}
	return strings.Compare(left, right)
}

func listRegistryTags(ctx context.Context, imageRef string) ([]string, error) {
	registry, repository := splitRegistryRepository(imageRef)
	endpoint := "https://" + registry + "/v2/" + repository + "/tags/list?n=1000"
	client := &http.Client{Timeout: 30 * time.Second}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("list registry tags for %q: %w", imageRef, err)
	}
	if response.StatusCode == http.StatusUnauthorized {
		authRequest, err := registryAuthRequest(ctx, client, response.Header.Get("WWW-Authenticate"))
		if err != nil {
			return nil, err
		}
		if authRequest != nil {
			if err := response.Body.Close(); err != nil {
				return nil, fmt.Errorf("close registry tags response for %q: %w", imageRef, err)
			}
			request, _ = http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
			request.Header.Set("Authorization", "Bearer "+authRequest.Token)
			response, err = client.Do(request)
			if err != nil {
				return nil, fmt.Errorf("list registry tags for %q: %w", imageRef, err)
			}
		}
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("list registry tags for %q returned %s", imageRef, response.Status)
	}
	var payload struct {
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode registry tags for %q: %w", imageRef, err)
	}
	return payload.Tags, nil
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
		return nil, nil
	}
	params := parseAuthChallenge(challenge[len("Bearer "):])
	realm := params["realm"]
	if realm == "" {
		return nil, fmt.Errorf("registry auth challenge is missing realm")
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

func collectServiceImageObservations(ctx context.Context, serviceDir string, compose composeCommandConfig, includeRemote bool) ([]serviceImageObservation, error) {
	config, err := loadComposeConfigOutput(ctx, serviceDir, compose)
	if err != nil {
		return nil, err
	}
	composeServices := make([]string, 0, len(config.Services))
	for composeService := range config.Services {
		composeServices = append(composeServices, composeService)
	}
	slices.Sort(composeServices)

	observations := make([]serviceImageObservation, 0, len(composeServices))
	for _, composeService := range composeServices {
		imageRef := strings.TrimSpace(config.Services[composeService].Image)
		if imageRef == "" {
			continue
		}
		observation := serviceImageObservation{ComposeService: composeService, ImageRef: imageRef}
		localDigest, err := inspectLocalImageDigest(ctx, imageRef)
		if err != nil {
			observation.ErrorSummary = appendImageObservationError(observation.ErrorSummary, fmt.Sprintf("local digest: %v", err))
		} else if localDigest != "" {
			observation.LocalDigest = localDigest
			observation.LocalObserved = true
		}
		if includeRemote {
			remoteDigest, err := inspectRemoteImageDigest(ctx, imageRef)
			if err != nil {
				observation.ErrorSummary = appendImageObservationError(observation.ErrorSummary, fmt.Sprintf("remote digest: %v", err))
			} else if remoteDigest != "" {
				observation.RemoteDigest = remoteDigest
				observation.RemoteObserved = true
			}
		}
		observations = append(observations, observation)
	}
	return observations, nil
}

func loadComposeConfigOutput(ctx context.Context, serviceDir string, compose composeCommandConfig) (composeConfigOutput, error) {
	command := exec.CommandContext(ctx, "docker", buildComposeArgs(compose, "config", "--format", "json")...)
	command.Dir = serviceDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return composeConfigOutput{}, fmt.Errorf("docker compose config failed: %w %s", err, strings.TrimSpace(stderr.String()))
	}
	var config composeConfigOutput
	if err := json.Unmarshal(stdout.Bytes(), &config); err != nil {
		return composeConfigOutput{}, fmt.Errorf("decode docker compose config json: %w", err)
	}
	return config, nil
}

func inspectLocalImageDigest(ctx context.Context, imageRef string) (string, error) {
	command := exec.CommandContext(ctx, "docker", "image", "inspect", "--format", "{{range .RepoDigests}}{{println .}}{{end}}", imageRef)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("docker image inspect failed: %w %s", err, strings.TrimSpace(stderr.String()))
	}
	return firstDigestFromRepoDigests(stdout.String()), nil
}

func inspectRemoteImageDigest(ctx context.Context, imageRef string) (string, error) {
	command := exec.CommandContext(ctx, "docker", "buildx", "imagetools", "inspect", "--format", "{{.Digest}}", imageRef)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("docker buildx imagetools inspect failed: %w %s", err, strings.TrimSpace(stderr.String()))
	}
	digest := strings.TrimSpace(stdout.String())
	if digest == "" || digest == "<no value>" {
		return "", fmt.Errorf("docker buildx imagetools inspect did not return a digest")
	}
	return normalizeImageDigest(digest), nil
}

func firstDigestFromRepoDigests(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return normalizeImageDigest(line)
	}
	return ""
}

func normalizeImageDigest(value string) string {
	value = strings.TrimSpace(value)
	if at := strings.LastIndex(value, "@"); at >= 0 {
		return value[at+1:]
	}
	return value
}

func appendImageObservationError(existing, next string) string {
	if existing == "" {
		return next
	}
	return existing + "; " + next
}
