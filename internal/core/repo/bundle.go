package repo

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	gzipWriter := gzip.NewWriter(writer)
	if err := streamServiceTarWithExtras(ctx, repoDir, revision, serviceDir, extras, gzipWriter); err != nil {
		_ = gzipWriter.Close()
		return err
	}
	return gzipWriter.Close()
}

func streamServiceTarWithExtras(ctx context.Context, repoDir, revision, serviceDir string, extras map[string]string, writer io.Writer) error {
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

	tarReader := tar.NewReader(stdout)
	tarWriter := tar.NewWriter(writer)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			_ = tarWriter.Close()
			waitAfterError()
			return fmt.Errorf("stream git archive entry: %w", err)
		}
		if header.Typeflag == tar.TypeReg && IsEncryptedFilePath(header.Name) {
			if _, replaced := normalizedExtras[RuntimeFilePath(header.Name)]; replaced {
				continue
			}
		}
		if _, replaced := normalizedExtras[header.Name]; replaced {
			_ = tarWriter.Close()
			waitAfterError()
			return fmt.Errorf("bundle file %q conflicts with an injected runtime file", header.Name)
		}
		clonedHeader := *header
		if err := tarWriter.WriteHeader(&clonedHeader); err != nil {
			_ = tarWriter.Close()
			waitAfterError()
			return fmt.Errorf("write bundle header %q: %w", header.Name, err)
		}
		if header.Typeflag == tar.TypeReg {
			if _, err := io.Copy(tarWriter, tarReader); err != nil { //nolint:gosec
				_ = tarWriter.Close()
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
			waitAfterError()
			return fmt.Errorf("write injected bundle header %q: %w", extraPath, err)
		}
		if _, err := tarWriter.Write(body); err != nil {
			_ = tarWriter.Close()
			waitAfterError()
			return fmt.Errorf("write injected bundle file %q: %w", extraPath, err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		waitAfterError()
		return fmt.Errorf("close tar writer: %w", err)
	}
	if err := command.Wait(); err != nil {
		return fmt.Errorf("wait for git archive: %w %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

type ServiceManifestFile struct {
	Path   string
	SHA256 string
	Mode   uint32
}

// ServiceManifest uses the deployment renderer so encrypted paths and permissions match installed files.
func ServiceManifest(ctx context.Context, repoDir, revision, serviceDir string, extras map[string]string) ([]ServiceManifestFile, error) {
	reader, writer := io.Pipe()
	finished := make(chan error, 1)
	go func() {
		err := streamServiceTarWithExtras(ctx, repoDir, revision, serviceDir, extras, writer)
		_ = writer.CloseWithError(err)
		finished <- err
	}()
	defer func() {
		_ = reader.Close()
		<-finished
	}()
	tarReader := tar.NewReader(reader)
	files := make([]ServiceManifestFile, 0)
	hash := sha256.New()
	buffer := make([]byte, 32*1024)
	prefix := path.Clean(serviceDir) + "/"
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			// Git can fail after emitting a complete tar stream.
			if _, err := io.Copy(io.Discard, reader); err != nil {
				return nil, err
			}
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag == tar.TypeDir || header.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		if header.Typeflag != tar.TypeReg || !strings.HasPrefix(header.Name, prefix) {
			return nil, fmt.Errorf("unsupported service manifest entry %q", header.Name)
		}
		hash.Reset()
		if _, err := io.CopyBuffer(hash, tarReader, buffer); err != nil { //nolint:gosec // This stream is generated locally by git, not decompressed input.
			return nil, err
		}
		files = append(files, ServiceManifestFile{Path: strings.TrimPrefix(header.Name, prefix), SHA256: hex.EncodeToString(hash.Sum(nil)), Mode: uint32(header.Mode & 0o777)})
	}
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
