package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

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
		if image.Filter != nil && image.Filter.Type == imageUpdateFilterSemver && len(image.Filter.Allow) == 0 && len(params.SemverAllow) > 0 {
			image.Filter.Allow = append([]string(nil), params.SemverAllow...)
		}
		checks = append(checks, collectServiceImageUpdateCheck(ctx, serviceDir, imageName, image, serviceMeta.Update.DiscoverySources, params.ForgeCandidates[imageName], params.ForgeCandidateSources[imageName]))
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

func collectServiceImageUpdateCheck(ctx context.Context, serviceDir, imageName string, image repo.ImageUpdateConfig, discoverySources map[string]repo.ImageUpdateDiscovery, injectedCandidates []string, injectedSourceCandidates map[string][]string) *agentv1.ServiceImageUpdateCheck {
	policyType := imageUpdatePolicyDigest
	if image.Filter != nil {
		policyType = image.Filter.Type
	}
	check := &agentv1.ServiceImageUpdateCheck{ImageName: imageName, ImageRef: image.Image, PolicyType: policyType, CheckStatus: store.ImageCheckStatusOK}
	currentValue, currentTag, currentDigest, err := currentImageUpdateValue(serviceDir, image)
	if err != nil {
		check.CheckStatus = store.ImageCheckStatusError
		check.ErrorSummary = err.Error()
		return check
	}
	check.CurrentValue = currentValue
	check.CurrentTag = currentTag
	check.CurrentDigest = currentDigest

	discovery := resolveImageUpdateDiscovery(image.Discovery, discoverySources)
	if repo.IsDigestImageDiscovery(discovery, nil) {
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

	tags, err := discoverImageUpdateTags(ctx, image.Image, currentTag, discovery, image.Filter, injectedCandidates, injectedSourceCandidates)
	if err != nil {
		check.CheckStatus = store.ImageCheckStatusError
		check.ErrorSummary = err.Error()
		return check
	}
	candidates := candidateImageTags(currentTag, tags, image.Filter)
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
	if image.Current.Tag != "" {
		return image.Current.Tag, image.Current.Tag, "", nil
	}
	if image.Current.Env == nil && image.Current.YAML == nil {
		return "", "", "", errors.New("image update current source is required")
	}
	var currentFile string
	if image.Current.Env != nil {
		currentFile = image.Current.Env.File
	} else {
		currentFile = image.Current.YAML.File
	}
	path := filepath.Join(serviceDir, filepath.FromSlash(currentFile))
	content, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return "", "", "", fmt.Errorf("read image update current %q: %w", currentFile, err)
	}
	if image.Current.Env != nil {
		value, err := envFileValue(string(content), image.Current.Env.Key)
		if err != nil {
			return "", "", "", err
		}
		tag, digest := splitTagDigest(value)
		return value, tag, digest, nil
	}
	if image.Current.YAML != nil {
		value, err := yamlPathStringValue(content, image.Current.YAML.Path)
		if err != nil {
			return "", "", "", err
		}
		tag, digest := splitImageRefTagDigest(value)
		return value, tag, digest, nil
	}
	return "", "", "", errors.New("image update current env or yaml source is required")
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
	command := exec.CommandContext(ctx, "docker", buildComposeArgs(compose, "config", "--format", "json")...) //nolint:gosec
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
	command := exec.CommandContext(ctx, "docker", dockerResourceImage, "inspect", "--format", "{{range .RepoDigests}}{{println .}}{{end}}", imageRef) //nolint:gosec
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
	command := exec.CommandContext(ctx, "docker", "buildx", "imagetools", "inspect", "--format", "{{.Manifest.Digest}}", imageRef) //nolint:gosec
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("docker buildx imagetools inspect failed: %w %s", err, strings.TrimSpace(stderr.String()))
	}
	digest := strings.TrimSpace(stdout.String())
	if digest == "" || digest == "<no value>" {
		return "", errors.New("docker buildx imagetools inspect did not return a digest")
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
