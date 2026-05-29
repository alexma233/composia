package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
)

type composeUpOptions struct {
	ForceRecreate bool
}

type composeConfigOutput struct {
	Services map[string]composeConfigService `json:"services"`
}

type composeConfigService struct {
	Image   string                `json:"image"`
	Volumes []composeConfigVolume `json:"volumes"`
}

type composeConfigVolume struct {
	Type   string `json:"type"`
	Source string `json:"source"`
	Target string `json:"target"`
}

type serviceDirBindMount struct {
	Service string
	Source  string
	Target  string
}

func runComposeUpForServiceTask(ctx context.Context, serviceDir string, compose composeCommandConfig, params controllerTaskParams, uploadLog func(string) error) error {
	forceRecreate, err := resolveComposeForceRecreate(ctx, serviceDir, compose, params.ComposeRecreateMode, uploadLog)
	if err != nil {
		return err
	}
	return runComposeUpWithOptions(ctx, serviceDir, compose, composeUpOptions{ForceRecreate: forceRecreate}, uploadLog)
}

func resolveComposeForceRecreate(ctx context.Context, serviceDir string, compose composeCommandConfig, mode string, uploadLog func(string) error) (bool, error) {
	normalizedMode := normalizeComposeRecreateMode(mode)
	if err := uploadLog(fmt.Sprintf("compose recreate mode=%s\n", normalizedMode)); err != nil {
		return false, err
	}
	if normalizedMode == composeRecreateForce {
		if err := uploadLog("using docker compose up -d --force-recreate by request\n"); err != nil {
			return false, err
		}
		return true, nil
	}

	mounts, err := detectServiceDirBindMounts(ctx, serviceDir, compose)
	if err != nil {
		if normalizedMode == composeRecreateAuto {
			return false, err
		}
		if err := uploadLog(fmt.Sprintf("warning: could not inspect compose bind mounts: %v\n", err)); err != nil {
			return false, err
		}
		return false, nil
	}
	if len(mounts) == 0 {
		return false, nil
	}
	if err := uploadLog("detected bind mounts from replaceable service directory:\n"); err != nil {
		return false, err
	}
	for _, mount := range mounts {
		if err := uploadLog(fmt.Sprintf("- %s: %s -> %s\n", mount.Service, mount.Source, mount.Target)); err != nil {
			return false, err
		}
	}
	if normalizedMode == composeRecreateNo {
		if err := uploadLog("warning: recreate is disabled by request; using docker compose up -d\n"); err != nil {
			return false, err
		}
		return false, nil
	}
	if err := uploadLog("using docker compose up -d --force-recreate to refresh bind mounts\n"); err != nil {
		return false, err
	}
	return true, nil
}

func normalizeComposeRecreateMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case composeRecreateNo, "no-recreate", "never":
		return composeRecreateNo
	case composeRecreateForce, "force-recreate", "always":
		return composeRecreateForce
	default:
		return composeRecreateAuto
	}
}

func detectServiceDirBindMounts(ctx context.Context, serviceDir string, compose composeCommandConfig) ([]serviceDirBindMount, error) {
	command := exec.CommandContext(ctx, "docker", buildComposeArgs(compose, "config", "--format", "json")...) //nolint:gosec
	command.Dir = serviceDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("docker compose config failed: %w %s", err, strings.TrimSpace(stderr.String()))
	}

	var config composeConfigOutput
	if err := json.Unmarshal(stdout.Bytes(), &config); err != nil {
		return nil, fmt.Errorf("decode docker compose config json: %w", err)
	}
	serviceRoot, err := filepath.Abs(serviceDir)
	if err != nil {
		return nil, fmt.Errorf("resolve service directory: %w", err)
	}
	mounts := make([]serviceDirBindMount, 0)
	for serviceName, service := range config.Services {
		for _, volume := range service.Volumes {
			if volume.Type != "bind" || strings.TrimSpace(volume.Source) == "" {
				continue
			}
			sourcePath := volume.Source
			if !filepath.IsAbs(sourcePath) {
				sourcePath = filepath.Join(serviceRoot, sourcePath)
			}
			sourcePath, err = filepath.Abs(sourcePath)
			if err != nil {
				return nil, fmt.Errorf("resolve bind source %q: %w", volume.Source, err)
			}
			if pathInside(serviceRoot, sourcePath) {
				mounts = append(mounts, serviceDirBindMount{Service: serviceName, Source: sourcePath, Target: volume.Target})
			}
		}
	}
	return mounts, nil
}

func loadComposeCommandConfig(serviceDir, fallback string) (composeCommandConfig, repo.ServiceMeta, error) {
	metaPath := filepath.Join(serviceDir, "composia-meta.yaml")
	meta, err := repo.LoadServiceMeta(metaPath)
	if err != nil {
		return composeCommandConfig{}, repo.ServiceMeta{}, err
	}
	composeFiles, err := meta.NormalizedComposeFiles()
	if err != nil {
		return composeCommandConfig{}, repo.ServiceMeta{}, fmt.Errorf("service meta %q: %w", metaPath, err)
	}
	return composeCommandConfig{ProjectName: repo.ComposeProjectName(meta.ProjectName, fallback), Files: composeFiles}, meta, nil
}
