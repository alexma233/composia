package repo

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

func StreamServiceBundle(ctx context.Context, repoDir, revision, serviceDir string, writer io.Writer) error {
	return StreamServiceBundleWithExtras(ctx, repoDir, revision, serviceDir, nil, writer)
}

func StreamServiceBundleWithExtras(ctx context.Context, repoDir, revision, serviceDir string, extras map[string]string, writer io.Writer) error {
	command := exec.CommandContext(ctx, "git", "-C", repoDir, "archive", "--format=tar", revision, serviceDir)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create git archive pipe: %w", err)
	}

	stderr := new(strings.Builder)
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return fmt.Errorf("start git archive: %w", err)
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
			_ = command.Wait()
			return fmt.Errorf("stream git archive entry: %w", err)
		}
		clonedHeader := *header
		if err := tarWriter.WriteHeader(&clonedHeader); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			_ = command.Wait()
			return fmt.Errorf("write bundle header %q: %w", header.Name, err)
		}
		if header.Typeflag == tar.TypeReg {
			if _, err := io.Copy(tarWriter, tarReader); err != nil {
				_ = tarWriter.Close()
				_ = gzipWriter.Close()
				_ = command.Wait()
				return fmt.Errorf("write bundle file %q: %w", header.Name, err)
			}
		}
	}
	for name, content := range extras {
		body := []byte(content)
		header := &tar.Header{Name: name, Mode: 0o600, Size: int64(len(body))}
		if err := tarWriter.WriteHeader(header); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			_ = command.Wait()
			return fmt.Errorf("write injected bundle header %q: %w", name, err)
		}
		if _, err := tarWriter.Write(body); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			_ = command.Wait()
			return fmt.Errorf("write injected bundle file %q: %w", name, err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		_ = command.Wait()
		return fmt.Errorf("close tar writer: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		_ = command.Wait()
		return fmt.Errorf("close gzip writer: %w", err)
	}
	if err := command.Wait(); err != nil {
		return fmt.Errorf("wait for git archive: %w %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
