package agent

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
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
	if err := os.MkdirAll(stageRoot, 0o755); err != nil {
		return "", fmt.Errorf("create data-protect stage root %q: %w", stageRoot, err)
	}
	stageDir, err := os.MkdirTemp(stageRoot, prefix)
	if err != nil {
		return "", fmt.Errorf("create data-protect stage dir: %w", err)
	}
	return stageDir, nil
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

func stageVolumeToDir(ctx context.Context, targetDir, volumeName string) error {
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("clear staged volume dir %q: %w", targetDir, err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create staged volume dir %q: %w", targetDir, err)
	}
	return runDockerVolumeTarExport(ctx, volumeName, targetDir)
}

func restoreDirToVolume(ctx context.Context, sourceDir, volumeName string) error {
	info, err := os.Stat(sourceDir)
	if err != nil {
		return fmt.Errorf("stat restore volume source %q: %w", sourceDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("restore volume source %q must be a directory", sourceDir)
	}
	return runDockerVolumeTarImport(ctx, sourceDir, volumeName)
}

func copyIntoStage(sourcePath, targetPath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat source path %q: %w", sourcePath, err)
	}
	if info.IsDir() {
		return copyDir(sourcePath, targetPath)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create stage dir for %q: %w", targetPath, err)
	}
	return copyFile(sourcePath, targetPath, info.Mode())
}

func copyDir(sourceDir, targetDir string) error {
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		targetPath := targetDir
		if relPath != "." {
			targetPath = filepath.Join(targetDir, relPath)
		}
		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}
		return copyFile(path, targetPath, info.Mode())
	})
}

func copyFile(sourcePath, targetPath string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		_ = sourceFile.Close()
		return err
	}
	if _, err = io.Copy(targetFile, sourceFile); err != nil {
		_ = targetFile.Close()
		_ = sourceFile.Close()
		return err
	}
	if err := targetFile.Close(); err != nil {
		_ = sourceFile.Close()
		return err
	}
	if err := sourceFile.Close(); err != nil {
		return err
	}
	return nil
}

func writeTarStream(sourceDir string, tarWriter *tar.Writer) error {
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		linkTarget := ""
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err = os.Readlink(path)
			if err != nil {
				return fmt.Errorf("read symlink %q: %w", path, err)
			}
		}
		header, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err = io.Copy(tarWriter, file); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		return nil
	})
}

func runDockerVolumeTarExport(ctx context.Context, volumeName, targetDir string) error {
	command := exec.CommandContext(ctx, "docker", "run", "--rm", "-v", volumeName+":/source:ro", dockerVolumeTarImage, "tar", "-C", "/source", "-cf", "-", ".")
	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("prepare docker volume export stdout for %q: %w", volumeName, err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return fmt.Errorf("start docker volume export for %q: %w", volumeName, err)
	}
	extractErr := extractTarStream(stdout, targetDir)
	if extractErr != nil {
		_ = stdout.Close()
	}
	waitErr := command.Wait()
	if extractErr != nil {
		if waitErr != nil {
			return fmt.Errorf("extract docker volume %q tar stream: %w (docker wait error: %v)", volumeName, extractErr, formatDockerRunError("docker run volume export failed", waitErr, stderr.String()))
		}
		return fmt.Errorf("extract docker volume %q tar stream: %w", volumeName, extractErr)
	}
	if waitErr != nil {
		return formatDockerRunError("docker run volume export failed", waitErr, stderr.String())
	}
	return nil
}

func runDockerVolumeTarImport(ctx context.Context, sourceDir, volumeName string) error {
	command := exec.CommandContext(ctx, "docker", "run", "-i", "--rm", "-v", volumeName+":/target", dockerVolumeTarImage, "sh", "-c", dockerVolumeImportCmd)
	stdin, err := command.StdinPipe()
	if err != nil {
		return fmt.Errorf("prepare docker volume import stdin for %q: %w", volumeName, err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return fmt.Errorf("start docker volume import for %q: %w", volumeName, err)
	}
	streamErr := writeTarToWriter(sourceDir, stdin)
	closeErr := stdin.Close()
	waitErr := command.Wait()
	if streamErr != nil {
		if waitErr != nil {
			return fmt.Errorf("write restore tar stream for docker volume %q: %w (docker wait error: %v)", volumeName, streamErr, formatDockerRunError("docker run volume import failed", waitErr, stderr.String()))
		}
		return fmt.Errorf("write restore tar stream for docker volume %q: %w", volumeName, streamErr)
	}
	if closeErr != nil {
		if waitErr != nil {
			return fmt.Errorf("close docker volume import stdin for %q: %w (docker wait error: %v)", volumeName, closeErr, formatDockerRunError("docker run volume import failed", waitErr, stderr.String()))
		}
		return fmt.Errorf("close docker volume import stdin for %q: %w", volumeName, closeErr)
	}
	if waitErr != nil {
		return formatDockerRunError("docker run volume import failed", waitErr, stderr.String())
	}
	return nil
}

func writeTarToWriter(sourceDir string, writer io.Writer) error {
	tarWriter := tar.NewWriter(writer)
	if err := writeTarStream(sourceDir, tarWriter); err != nil {
		_ = tarWriter.Close()
		return err
	}
	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("close tar stream: %w", err)
	}
	return nil
}

func formatDockerRunError(prefix string, err error, stderr string) error {
	trimmed := strings.TrimSpace(stderr)
	if trimmed == "" {
		return fmt.Errorf("%s: %w", prefix, err)
	}
	return fmt.Errorf("%s: %w: %s", prefix, err, trimmed)
}

func runComposePGDumpAll(ctx context.Context, serviceDir string, compose composeCommandConfig, serviceName, targetPath string, uploadLog func(string) error) error {
	if serviceName == "" {
		return fmt.Errorf("pgdumpall backup is missing service name")
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create pgdump target dir: %w", err)
	}
	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create pgdump target file: %w", err)
	}
	command := exec.CommandContext(ctx, "docker", buildComposeArgs(compose, "exec", "-T", serviceName, "pg_dumpall")...)
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
		return fmt.Errorf("pgimport restore is missing service name")
	}
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open pgimport source file: %w", err)
	}
	command := exec.CommandContext(ctx, "docker", buildComposeArgs(compose, "exec", "-T", serviceName, "psql")...)
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
