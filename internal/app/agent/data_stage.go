package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	backupcfg "forgejo.alexma.top/alexma233/composia/internal/core/backup"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
)

func dataProtectStageRoot(stateDir string) string {
	return filepath.Join(stateDir, "data-protect")
}

func dataProtectStageDir(stateDir, prefix string) (string, error) {
	stageRoot := dataProtectStageRoot(stateDir)
	if err := os.MkdirAll(stageRoot, 0o750); err != nil {
		return "", fmt.Errorf("create data-protect stage root %q: %w", stageRoot, err)
	}
	stageDir, err := os.MkdirTemp(stageRoot, prefix)
	if err != nil {
		return "", fmt.Errorf("create data-protect stage dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(stageDir, "paths"), 0o750); err != nil {
		return "", fmt.Errorf("create data-protect stage paths dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(stageDir, "volumes"), 0o750); err != nil {
		return "", fmt.Errorf("create data-protect stage volumes dir: %w", err)
	}
	return stageDir, nil
}

func clearDockerVolume(ctx context.Context, volumeName string) error {
	command := exec.CommandContext(ctx, "docker", "run", "--rm", "-v", volumeName+":/target", "alpine:3.20", "sh", "-c", "rm -rf /target/..?* /target/.[!.]* /target/*") //nolint:gosec
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("clear docker volume %q: %w: %s", volumeName, err, string(output))
	}
	return nil
}

func rusticDataProtectPath(localPath string, cfg *config.AgentConfig, rustic *backupcfg.RusticConfig) (string, error) {
	if rustic == nil || strings.TrimSpace(rustic.DataProtectDir) == "" {
		return localPath, nil
	}
	stageRoot := dataProtectStageRoot(cfg.StateDir)
	relativePath, err := filepath.Rel(stageRoot, localPath)
	if err != nil {
		return "", fmt.Errorf("resolve relative data-protect path for %q: %w", localPath, err)
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside agent data-protect stage root %q", localPath, stageRoot)
	}
	return filepath.Join(rustic.DataProtectDir, relativePath), nil
}

func runComposePGDumpAll(ctx context.Context, serviceDir string, compose composeCommandConfig, serviceName, targetPath string, uploadLog func(string) error) error {
	if serviceName == "" {
		return errors.New("pgdumpall backup is missing service name")
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o750); err != nil {
		return fmt.Errorf("create pgdump target dir: %w", err)
	}
	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //nolint:gosec
	if err != nil {
		return fmt.Errorf("create pgdump target file: %w", err)
	}
	command := exec.CommandContext(ctx, "docker", buildComposeArgs(compose, "exec", "-T", serviceName, "pg_dumpall")...) //nolint:gosec
	command.Dir = serviceDir
	command.Stdout = targetFile
	command.Stderr = newCommandLogWriter(uploadLog, false)
	err = command.Run()
	if err != nil {
		_ = targetFile.Close()
		return fmt.Errorf("docker compose exec pg_dumpall failed: %w", err)
	}
	if err := targetFile.Close(); err != nil {
		return fmt.Errorf("close pgdump target file: %w", err)
	}
	return nil
}

func runComposePGImport(ctx context.Context, serviceDir string, compose composeCommandConfig, serviceName, sourcePath string, uploadLog func(string) error) error {
	if serviceName == "" {
		return errors.New("pgimport restore is missing service name")
	}
	sourceFile, err := os.Open(sourcePath) //nolint:gosec
	if err != nil {
		return fmt.Errorf("open pgimport source file: %w", err)
	}
	command := exec.CommandContext(ctx, "docker", buildComposeArgs(compose, "exec", "-T", serviceName, "psql")...) //nolint:gosec
	command.Dir = serviceDir
	command.Stdin = sourceFile
	if err := runCommandWithLiveLogs(command, uploadLog); err != nil {
		_ = sourceFile.Close()
		return fmt.Errorf("docker compose exec psql failed: %w", err)
	}
	if err := sourceFile.Close(); err != nil {
		return fmt.Errorf("close pgimport source file: %w", err)
	}
	return nil
}
