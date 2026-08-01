package repo

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"slices"
	"strings"
)

func StreamServiceBundle(ctx context.Context, repoDir, revision, serviceDir string, writer io.Writer) error {
	return StreamServiceBundleWithExtras(ctx, repoDir, revision, serviceDir, nil, writer)
}

func StreamServiceBundleWithExtras(ctx context.Context, repoDir, revision, serviceDir string, extras map[string]string, writer io.Writer) error {
	normalizedExtras := make(map[string]string, len(extras))
	for name, content := range extras {
		extraPath, err := normalizeBundleExtraPath(name)
		if err != nil {
			return err
		}
		normalizedExtras[extraPath] = content
	}
	commandCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	command := exec.CommandContext(commandCtx, "git", "-C", repoDir, "archive", "--format=tar", revision, serviceDir) //nolint:gosec
	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create git archive pipe: %w", err)
	}

	stderr := new(strings.Builder)
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return fmt.Errorf("start git archive: %w", err)
	}
	waitAfterError := func() {
		cancel()
		_ = command.Wait()
	}

	gzipWriter := gzip.NewWriter(writer)
	tarReader := tar.NewReader(stdout)
	tarWriter := tar.NewWriter(gzipWriter)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			waitAfterError()
			return fmt.Errorf("stream git archive entry: %w", err)
		}
		if header.Typeflag == tar.TypeReg && IsEncryptedFilePath(header.Name) {
			if _, replaced := normalizedExtras[RuntimeFilePath(header.Name)]; replaced {
				if _, err := io.Copy(io.Discard, tarReader); err != nil {
					_ = tarWriter.Close()
					_ = gzipWriter.Close()
					waitAfterError()
					return fmt.Errorf("skip encrypted bundle file %q: %w", header.Name, err)
				}
				continue
			}
		}
		if _, replaced := normalizedExtras[header.Name]; replaced {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			waitAfterError()
			return fmt.Errorf("bundle file %q conflicts with an injected runtime file", header.Name)
		}
		clonedHeader := *header
		if err := tarWriter.WriteHeader(&clonedHeader); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			waitAfterError()
			return fmt.Errorf("write bundle header %q: %w", header.Name, err)
		}
		if header.Typeflag == tar.TypeReg {
			if _, err := io.Copy(tarWriter, tarReader); err != nil { //nolint:gosec
				_ = tarWriter.Close()
				_ = gzipWriter.Close()
				waitAfterError()
				return fmt.Errorf("write bundle file %q: %w", header.Name, err)
			}
		}
	}
	extraPaths := make([]string, 0, len(normalizedExtras))
	for name := range normalizedExtras {
		extraPaths = append(extraPaths, name)
	}
	slices.Sort(extraPaths)
	for _, extraPath := range extraPaths {
		content := normalizedExtras[extraPath]
		body := []byte(content)
		header := &tar.Header{Name: extraPath, Mode: 0o600, Size: int64(len(body))}
		if err := tarWriter.WriteHeader(header); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			waitAfterError()
			return fmt.Errorf("write injected bundle header %q: %w", extraPath, err)
		}
		if _, err := tarWriter.Write(body); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			waitAfterError()
			return fmt.Errorf("write injected bundle file %q: %w", extraPath, err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		waitAfterError()
		return fmt.Errorf("close tar writer: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		waitAfterError()
		return fmt.Errorf("close gzip writer: %w", err)
	}
	if err := command.Wait(); err != nil {
		return fmt.Errorf("wait for git archive: %w %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func normalizeBundleExtraPath(name string) (string, error) {
	cleanName := path.Clean(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	if cleanName == "" || cleanName == "." {
		return "", errors.New("bundle extra path must not be empty")
	}
	if strings.HasPrefix(cleanName, "/") || cleanName == ".." || strings.HasPrefix(cleanName, "../") {
		return "", fmt.Errorf("bundle extra path %q escapes bundle root", name)
	}
	return cleanName, nil
}
