package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	"github.com/distribution/reference"
)

type consistencyDriftError struct{ error }

func driftf(format string, args ...any) error {
	return consistencyDriftError{fmt.Errorf(format, args...)}
}

func consistencyOutcome(err error) *agentv1.ServiceConsistencyOutcome {
	outcome := &agentv1.ServiceConsistencyOutcome{Status: store.ConsistencyConsistent}
	if err != nil {
		outcome.Status = store.ConsistencyError
		var drift consistencyDriftError
		if errors.As(err, &drift) {
			outcome.Status = store.ConsistencyDrifted
		}
		outcome.Reasons = []string{err.Error()}
	}
	return outcome
}

// checkServiceConsistency reports configuration before any runtime or registry observation.
func checkServiceConsistency(ctx context.Context, bundles agentv1connect.BundleServiceClient, reports agentv1connect.AgentReportServiceClient, pulledTask *agentv1.AgentTask, serviceRoot string) (meta repo.ServiceMeta, containers *checkedServiceContainers, err error) {
	report := &agentv1.ReportServiceConsistencyCheckRequest{
		TaskId: pulledTask.GetTaskId(), ExecutionId: taskExecutionID(ctx),
		Files: &agentv1.ServiceConsistencyOutcome{Status: store.ConsistencyUnknown}, Compose: &agentv1.ServiceConsistencyOutcome{Status: store.ConsistencyUnknown},
	}
	defer func() {
		_, reportErr := reports.ReportServiceConsistencyCheck(ctx, connect.NewRequest(report))
		err = errors.Join(err, reportErr)
	}()
	err = checkServiceFiles(ctx, bundles, pulledTask, serviceRoot)
	report.Files = consistencyOutcome(err)
	if err != nil {
		return
	}
	meta, err = loadServiceTaskMeta(serviceRoot)
	if err != nil {
		report.Compose = consistencyOutcome(err)
		return
	}
	if meta.IsConfigInfra() {
		report.Compose.Status = store.ConsistencyNotApplicable
		return
	}
	compose, _, err := loadComposeCommandConfig(serviceRoot, pulledTask.GetServiceName())
	if err == nil {
		containers, err = checkServiceContainers(ctx, serviceRoot, compose)
	}
	report.Compose = consistencyOutcome(err)
	return meta, containers, err
}

func checkServiceFiles(ctx context.Context, client agentv1connect.BundleServiceClient, pulledTask *agentv1.AgentTask, serviceRoot string) error {
	response, err := client.GetServiceManifest(ctx, connect.NewRequest(&agentv1.GetServiceManifestRequest{TaskId: pulledTask.GetTaskId(), ExecutionId: taskExecutionID(ctx)}))
	if err != nil {
		return fmt.Errorf("get service manifest: %w", err)
	}
	manifest := response.Msg
	if manifest.GetRepoRevision() != pulledTask.GetRepoRevision() || manifest.GetRelativeRoot() != filepath.ToSlash(pulledTask.GetServiceDir()) {
		return errors.New("service manifest does not match the task revision and directory")
	}
	return verifyServiceFiles(serviceRoot, manifest.GetFiles())
}

func verifyServiceFiles(serviceRoot string, files []*agentv1.ServiceManifestFile) error {
	root, err := os.OpenRoot(serviceRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return consistencyDriftError{err}
		}
		return err
	}
	defer func() { _ = root.Close() }()
	seen := make(map[string]bool, len(files))
	hash := sha256.New()
	buffer := make([]byte, 32*1024)
	for _, file := range files {
		name := file.GetPath()
		if !fs.ValidPath(name) || name == "." || seen[name] || len(file.GetSha256()) != sha256.Size*2 || file.GetMode() > 0o777 {
			return fmt.Errorf("invalid service manifest entry %q", name)
		}
		seen[name] = true
		info, err := root.Lstat(name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return driftf("service file %q is missing; deploy or update first: %w", name, err)
			}
			return fmt.Errorf("inspect service file %q: %w", name, err)
		}
		// Extraction applies the agent's umask; tighter permissions are valid, extra permissions are not.
		mode := uint32(info.Mode().Perm())
		if !info.Mode().IsRegular() || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || mode & ^file.GetMode() != 0 || mode&0o100 != file.GetMode()&0o100 {
			return driftf("service file %q has different type or permissions; deploy or update first", name)
		}
		input, err := root.Open(name)
		if err != nil {
			return err
		}
		hash.Reset()
		// Avoid File.WriteTo allocating a separate copy buffer for every small config file.
		_, readErr := io.CopyBuffer(hash, struct{ io.Reader }{input}, buffer)
		closeErr := input.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return fmt.Errorf("hash service file %q: %w", name, err)
		}
		if hex.EncodeToString(hash.Sum(nil)) != file.GetSha256() {
			return driftf("service file %q differs from the controller; deploy or update first", name)
		}
	}
	if !seen[repo.MetaFileName] {
		return errors.New("service manifest is missing composia-meta.yaml")
	}
	// An untracked .env changes Compose interpolation even when every managed file matches.
	if !seen[".env"] {
		if _, err := root.Lstat(".env"); err == nil {
			return driftf("local .env is not managed by the controller; remove it or add it to the repo before checking consistency")
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

type serviceCheckContainer struct {
	ID     string `json:"Id"`
	Image  string `json:"Image"`
	Config struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	State struct {
		Status string `json:"Status"`
	} `json:"State"`
}

func serviceDockerOutput(ctx context.Context, serviceRoot string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "docker", args...) //nolint:gosec
	command.Dir = serviceRoot
	output, err := command.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("docker %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	return output, nil
}

type checkedServiceContainers struct {
	config    composeConfigOutput
	byService map[string][]serviceCheckContainer
	services  []string
}

// checkServiceContainers uses the same CLI endpoint and model as deployment, including externally created Compose containers.
func checkServiceContainers(ctx context.Context, serviceRoot string, compose composeCommandConfig) (*checkedServiceContainers, error) {
	output, err := serviceDockerOutput(ctx, serviceRoot, "compose", "version", "--short")
	if err != nil {
		return nil, err
	}
	version, ok := parseSimpleSemver(strings.TrimSpace(string(output)))
	minimum, _ := parseSimpleSemver("5.5.1")
	if !ok || version.compare(minimum) < 0 {
		return nil, fmt.Errorf("docker compose 5.5.1 or newer is required for configuration checks (found %q)", strings.TrimSpace(string(output)))
	}
	output, err = serviceDockerOutput(ctx, serviceRoot, buildComposeArgs(compose, "--profile", "*", "config", "--format", "json")...)
	if err != nil {
		return nil, err
	}
	var config composeConfigOutput
	if err := json.Unmarshal(output, &config); err != nil {
		return nil, fmt.Errorf("decode compose configuration: %w", err)
	}
	output, err = serviceDockerOutput(ctx, serviceRoot, buildComposeArgs(compose, "--profile", "*", "config", "--hash", "*")...)
	if err != nil {
		return nil, err
	}
	hashes := make(map[string]string, len(config.Services))
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || len(fields[1]) != sha256.Size*2 || hashes[fields[0]] != "" {
			return nil, errors.New("invalid docker compose config --hash output")
		}
		hashes[fields[0]] = fields[1]
	}
	for name := range config.Services {
		if hashes[name] == "" {
			return nil, fmt.Errorf("compose config hash is missing for service %q", name)
		}
	}
	output, err = serviceDockerOutput(ctx, serviceRoot, "container", "ls", "--all", "--quiet", "--filter", "label=com.docker.compose.project="+compose.ProjectName)
	if err != nil {
		return nil, err
	}
	var containers []serviceCheckContainer
	if ids := strings.Fields(string(output)); len(ids) > 0 {
		output, err = serviceDockerOutput(ctx, serviceRoot, append([]string{"container", "inspect"}, ids...)...)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(output, &containers); err != nil {
			return nil, fmt.Errorf("decode compose containers: %w", err)
		}
	}
	byService := make(map[string][]serviceCheckContainer)
	for _, container := range containers {
		labels := container.Config.Labels
		if labels["com.docker.compose.project"] != compose.ProjectName || strings.EqualFold(labels["com.docker.compose.oneoff"], "true") {
			continue
		}
		service := labels["com.docker.compose.service"]
		if _, ok := config.Services[service]; !ok {
			return nil, driftf("container %q belongs to undeclared compose service %q; reconcile the deployment first", container.ID, service)
		}
		if labels["com.docker.compose.config-hash"] != hashes[service] {
			return nil, driftf("compose configuration for container %q is not applied; deploy or update first", container.ID)
		}
		byService[service] = append(byService[service], container)
	}
	services := make([]string, 0, len(config.Services))
	for name, service := range config.Services {
		if len(byService[name]) == 0 {
			if len(service.Profiles) > 0 {
				continue
			}
			return nil, driftf("compose service %q has no container; deploy or update first", name)
		}
		if service.Image != "" {
			services = append(services, name)
		}
	}
	slices.Sort(services)
	return &checkedServiceContainers{config: config, byService: byService, services: services}, nil
}

func observeServiceImages(ctx context.Context, serviceRoot string, checked *checkedServiceContainers) ([]serviceImageObservation, error) {
	for _, containers := range checked.byService {
		for _, container := range containers {
			if container.State.Status != "running" || container.Image == "" {
				return nil, fmt.Errorf("container %q has no running image observation (state=%s)", container.ID, container.State.Status)
			}
		}
	}
	services, config, byService := checked.services, checked.config, checked.byService
	observations := make([]serviceImageObservation, 0, len(services))
	for _, name := range services {
		imageRef := config.Services[name].Image
		observation := serviceImageObservation{ComposeService: name, ImageRef: imageRef, LocalObserved: true}
		for _, container := range byService[name] {
			output, err := serviceDockerOutput(ctx, serviceRoot, "image", "inspect", "--format", "{{json .RepoDigests}}", container.Image)
			if err != nil {
				return nil, err
			}
			var repoDigests []string
			if err := json.Unmarshal(output, &repoDigests); err != nil {
				return nil, fmt.Errorf("decode running image digests: %w", err)
			}
			digest, err := runningImageDigest(imageRef, repoDigests)
			if err != nil {
				return nil, fmt.Errorf("container %q: %w", container.ID, err)
			}
			if observation.LocalDigest != "" && observation.LocalDigest != digest {
				return nil, fmt.Errorf("compose service %q has replicas running different image digests", name)
			}
			observation.LocalDigest = digest
		}
		observations = append(observations, observation)
	}
	return observations, nil
}

func runningImageDigest(imageRef string, repoDigests []string) (string, error) {
	configured, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return "", err
	}
	matches := make(map[string]bool)
	for _, value := range repoDigests {
		observed, err := reference.ParseNormalizedNamed(value)
		if err != nil || reference.FamiliarName(reference.TrimNamed(observed)) != reference.FamiliarName(reference.TrimNamed(configured)) {
			continue
		}
		if digested, ok := observed.(reference.Digested); ok {
			matches[digested.Digest().String()] = true
		}
	}
	if pinned, ok := configured.(reference.Digested); ok {
		if matches[pinned.Digest().String()] {
			return pinned.Digest().String(), nil
		}
		return "", fmt.Errorf("running image does not match configured digest for %q", imageRef)
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("running image for %q has no unambiguous repository digest", imageRef)
	}
	for digest := range matches {
		return digest, nil
	}
	return "", errors.New("running image digest is unavailable")
}
