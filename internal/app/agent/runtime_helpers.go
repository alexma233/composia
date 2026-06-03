package agent

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
)

type composeCommandConfig struct {
	ProjectName string
	Files       []string
}

func buildComposeArgs(config composeCommandConfig, commandArgs ...string) []string {
	args := make([]string, 0, 3+2*len(config.Files)+len(commandArgs))
	args = append(args, "compose", "--project-name", config.ProjectName)
	for _, file := range config.Files {
		args = append(args, "-f", file)
	}
	args = append(args, commandArgs...)
	return args
}

func buildRusticComposeRunArgs(config composeCommandConfig, composeService, profile string, extraRunOpts []string, commandArgs ...string) []string {
	args := buildComposeArgs(config, "run", "--rm")
	args = append(args, extraRunOpts...)
	args = append(args, composeService)
	if profile != "" {
		args = append(args, "-P", profile)
	}
	args = append(args, commandArgs...)
	return args
}

func runComposeUp(ctx context.Context, serviceDir string, compose composeCommandConfig, uploadLog func(string) error) error {
	return runComposeUpWithOptions(ctx, serviceDir, compose, composeUpOptions{}, uploadLog)
}

func runComposeUpWithOptions(ctx context.Context, serviceDir string, compose composeCommandConfig, options composeUpOptions, uploadLog func(string) error) error {
	args := buildComposeArgs(compose, "up", "-d")
	if options.ForceRecreate {
		args = append(args, "--force-recreate")
	}
	command := exec.CommandContext(ctx, "docker", args...) //nolint:gosec
	command.Dir = serviceDir
	if err := runCommandWithLiveLogs(command, uploadLog); err != nil {
		return fmt.Errorf("docker compose up failed: %w", err)
	}
	return nil
}

func runComposeDown(ctx context.Context, serviceDir string, compose composeCommandConfig, uploadLog func(string) error) error {
	command := exec.CommandContext(ctx, "docker", buildComposeArgs(compose, "down")...) //nolint:gosec
	command.Dir = serviceDir
	if err := runCommandWithLiveLogs(command, uploadLog); err != nil {
		return fmt.Errorf("docker compose down failed: %w", err)
	}
	return nil
}

func runComposePull(ctx context.Context, serviceDir string, compose composeCommandConfig, uploadLog func(string) error) error {
	command := exec.CommandContext(ctx, "docker", buildComposeArgs(compose, "pull")...) //nolint:gosec
	command.Dir = serviceDir
	if err := runCommandWithLiveLogs(command, uploadLog); err != nil {
		return fmt.Errorf("docker compose pull failed: %w", err)
	}
	return nil
}

func collectRuntimeSummary(path string) (*agentv1.NodeRuntimeSummary, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, fmt.Errorf("read filesystem stats for %q: %w", path, err)
	}

	blockSize := uint64(stat.Bsize) //nolint:gosec
	dockerVersion := dockerServerVersion()

	return &agentv1.NodeRuntimeSummary{
		DockerServerVersion: dockerVersion,
		DiskTotalBytes:      stat.Blocks * blockSize,
		DiskFreeBytes:       stat.Bavail * blockSize,
	}, nil
}

func collectDockerStats(ctx context.Context) (*agentv1.DockerStats, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var stats agentv1.DockerStats
	stats.DockerServerVersion = dockerServerVersion()

	containers, err := dockerContainerStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect container stats: %w", err)
	}
	stats.ContainersTotal = containers.total
	stats.ContainersRunning = containers.running
	stats.ContainersStopped = containers.stopped
	stats.ContainersPaused = containers.paused

	images, err := dockerImageCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect image count: %w", err)
	}
	stats.Images = images

	networks, err := dockerNetworkCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect network count: %w", err)
	}
	stats.Networks = networks

	volumes, volumesSize, err := dockerVolumeStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect volume stats: %w", err)
	}
	stats.Volumes = volumes
	stats.VolumesSizeBytes = volumesSize

	stats.DisksUsageBytes, err = dockerDiskUsage(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect disk usage: %w", err)
	}

	return &stats, nil
}

type containerStats struct {
	total   uint32
	running uint32
	stopped uint32
	paused  uint32
}

func dockerContainerStats(ctx context.Context) (containerStats, error) {
	output, err := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "{{.State}}").Output()
	if err != nil {
		return containerStats{}, fmt.Errorf("docker ps failed: %w", err)
	}

	var stats containerStats
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		switch strings.TrimSpace(line) {
		case "running":
			stats.running++
		case "exited":
			stats.stopped++
		case "paused":
			stats.paused++
		}
	}
	stats.total = stats.running + stats.stopped + stats.paused
	return stats, nil
}

func dockerImageCount(ctx context.Context) (uint32, error) {
	output, err := exec.CommandContext(ctx, "docker", "images", "-q").Output()
	if err != nil {
		return 0, fmt.Errorf("docker images failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	count := uint32(0)
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, nil
}

func dockerNetworkCount(ctx context.Context) (uint32, error) {
	output, err := exec.CommandContext(ctx, "docker", "network", "ls", "-q").Output()
	if err != nil {
		return 0, fmt.Errorf("docker network ls failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	count := uint32(0)
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, nil
}

func dockerVolumeStats(ctx context.Context) (uint32, uint64, error) {
	output, err := exec.CommandContext(ctx, "docker", "volume", "ls", "-q").Output()
	if err != nil {
		return 0, 0, fmt.Errorf("docker volume ls failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	count := uint32(0)
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}

	sizeOutput, err := exec.CommandContext(ctx, "docker", "system", "df", "--format", "{{.Type}}\t{{.Size}}").Output()
	if err != nil {
		return count, 0, fmt.Errorf("docker system df failed: %w", err)
	}

	return count, parseDockerSystemDFVolumeSize(sizeOutput), nil
}

func parseDockerSystemDFVolumeSize(output []byte) uint64 {
	var totalSize uint64
	sizeLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range sizeLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		kind, sizeText, ok := strings.Cut(line, "\t")
		if !ok || !strings.Contains(strings.ToLower(kind), "volume") {
			continue
		}
		if size, ok := parseSize(sizeText); ok {
			totalSize += size
		}
	}
	return totalSize
}

func parseSize(s string) (uint64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}

	if strings.Contains(s, ",") {
		if strings.Contains(s, ".") {
			s = strings.ReplaceAll(s, ",", "")
		} else {
			s = strings.ReplaceAll(s, ",", ".")
		}
	}

	s = strings.ToUpper(s)

	unitMultipliers := []struct {
		suffix string
		bytes  float64
	}{
		{suffix: "PIB", bytes: 1024 * 1024 * 1024 * 1024 * 1024},
		{suffix: "PB", bytes: 1000 * 1000 * 1000 * 1000 * 1000},
		{suffix: "TIB", bytes: 1024 * 1024 * 1024 * 1024},
		{suffix: "TB", bytes: 1000 * 1000 * 1000 * 1000},
		{suffix: "GIB", bytes: 1024 * 1024 * 1024},
		{suffix: "GB", bytes: 1000 * 1000 * 1000},
		{suffix: "MIB", bytes: 1024 * 1024},
		{suffix: "MB", bytes: 1000 * 1000},
		{suffix: "KIB", bytes: 1024},
		{suffix: "KB", bytes: 1000},
		{suffix: "B", bytes: 1},
	}

	mult := float64(1)
	for _, unit := range unitMultipliers {
		if strings.HasSuffix(s, unit.suffix) {
			mult = unit.bytes
			s = strings.TrimSpace(strings.TrimSuffix(s, unit.suffix))
			break
		}
	}

	value, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || value < 0 {
		return 0, false
	}

	return uint64(math.Round(value * mult)), true
}

func dockerDiskUsage(ctx context.Context) (uint64, error) {
	output, err := exec.CommandContext(ctx, "docker", "system", "df", "--format", "{{.Size}}").Output()
	if err != nil {
		return 0, err
	}

	var total uint64
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "0B" {
			continue
		}
		if size, ok := parseSize(line); ok {
			total += size
		}
	}
	return total, nil
}

func dockerServerVersion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	output, err := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		return ""
	}
	return string(bytesTrimSpace(output))
}

func bytesTrimSpace(value []byte) []byte {
	start := 0
	for start < len(value) && (value[start] == ' ' || value[start] == '\n' || value[start] == '\t' || value[start] == '\r') {
		start++
	}

	end := len(value)
	for end > start && (value[end-1] == ' ' || value[end-1] == '\n' || value[end-1] == '\t' || value[end-1] == '\r') {
		end--
	}
	return value[start:end]
}

func pathInside(parent, child string) bool {
	relative, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func resolveRepoRelativePath(repoDir, relativePath, fieldName string) (string, string, error) {
	cleanRelativePath := filepath.Clean(strings.TrimSpace(relativePath))
	if cleanRelativePath == "" || cleanRelativePath == "." {
		return "", "", fmt.Errorf("%s must reference a service directory", fieldName)
	}
	if filepath.IsAbs(cleanRelativePath) {
		return "", "", fmt.Errorf("%s %q must be relative", fieldName, relativePath)
	}
	if cleanRelativePath == ".." || strings.HasPrefix(cleanRelativePath, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("%s %q escapes repo root", fieldName, relativePath)
	}

	absoluteRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve repo_dir %q: %w", repoDir, err)
	}
	absolutePath, err := filepath.Abs(filepath.Join(absoluteRepoDir, cleanRelativePath))
	if err != nil {
		return "", "", fmt.Errorf("resolve %s %q: %w", fieldName, relativePath, err)
	}
	if !pathInside(absoluteRepoDir, absolutePath) {
		return "", "", fmt.Errorf("%s %q escapes repo root", fieldName, relativePath)
	}
	return absolutePath, filepath.ToSlash(cleanRelativePath), nil
}
