package agent

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	securejoin "github.com/cyphar/filepath-securejoin"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
)

type bundleResult struct {
	ServiceName  string
	RelativeRoot string
	RootPath     string
}

func downloadServiceBundle(ctx context.Context, client agentv1connect.BundleServiceClient, cfg *config.AgentConfig, taskID, serviceDir string) (*bundleResult, error) {
	stream, err := client.GetServiceBundle(ctx, connect.NewRequest(&agentv1.GetServiceBundleRequest{TaskId: taskID, ServiceDir: serviceDir, ExecutionId: taskExecutionID(ctx)}))
	if err != nil {
		return nil, fmt.Errorf("get service bundle: %w", err)
	}
	defer func() { _ = stream.Close() }()

	tempFile, err := os.CreateTemp(cfg.StateDir, "bundle-*.tar.gz")
	if err != nil {
		return nil, fmt.Errorf("create temp bundle file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() { _ = os.Remove(tempPath) }()
	defer func() {
		if tempFile != nil {
			_ = tempFile.Close()
		}
	}()

	result := &bundleResult{}
	var relativeRoot string
	for stream.Receive() {
		message := stream.Msg()
		if result.ServiceName == "" && message.GetServiceName() != "" {
			result.ServiceName = message.GetServiceName()
		}
		if relativeRoot == "" && message.GetRelativeRoot() != "" {
			relativeRoot = message.GetRelativeRoot()
			result.RelativeRoot = relativeRoot
		}
		if _, err := tempFile.Write(message.GetData()); err != nil {
			return nil, fmt.Errorf("write temp bundle file: %w", err)
		}
	}
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("receive service bundle: %w", err)
	}
	if relativeRoot == "" {
		return nil, errors.New("bundle stream did not include relative_root metadata")
	}
	targetRoot, cleanRelativeRoot, err := resolveRepoRelativePath(cfg.RepoDir, relativeRoot, "relative_root")
	if err != nil {
		return nil, err
	}
	result.RelativeRoot = cleanRelativeRoot
	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp bundle file: %w", err)
	}
	tempFile = nil

	stageParentDir := filepath.Dir(targetRoot)
	if err := os.MkdirAll(stageParentDir, 0o750); err != nil {
		return nil, fmt.Errorf("create bundle stage parent dir %q: %w", stageParentDir, err)
	}
	stageDir, err := os.MkdirTemp(stageParentDir, ".bundle-stage-*")
	if err != nil {
		return nil, fmt.Errorf("create bundle stage dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(stageDir) }()
	if err := extractTarGz(tempPath, stageDir); err != nil {
		return nil, err
	}
	stagedRoot := filepath.Join(stageDir, cleanRelativeRoot)
	if _, err := os.Stat(stagedRoot); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("bundle archive did not contain expected root %q", cleanRelativeRoot)
		}
		return nil, fmt.Errorf("stat staged bundle root %q: %w", stagedRoot, err)
	}
	if err := replaceDirectory(targetRoot, stagedRoot); err != nil {
		return nil, err
	}
	result.RootPath = targetRoot
	return result, nil
}

func replaceDirectory(targetRoot, stagedRoot string) error {
	parentDir := filepath.Dir(targetRoot)
	if err := os.MkdirAll(parentDir, 0o750); err != nil {
		return fmt.Errorf("create bundle parent dir %q: %w", parentDir, err)
	}
	backupRoot := targetRoot + ".bak"
	if err := os.RemoveAll(backupRoot); err != nil {
		return fmt.Errorf("remove old bundle backup %q: %w", backupRoot, err)
	}
	hadExisting := false
	if _, err := os.Stat(targetRoot); err == nil {
		hadExisting = true
		if err := os.Rename(targetRoot, backupRoot); err != nil {
			return fmt.Errorf("move existing bundle %q to backup: %w", targetRoot, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat existing bundle root %q: %w", targetRoot, err)
	}
	if err := os.Rename(stagedRoot, targetRoot); err != nil {
		if hadExisting {
			_ = os.Rename(backupRoot, targetRoot)
		}
		return fmt.Errorf("activate staged bundle %q: %w", targetRoot, err)
	}
	if hadExisting {
		if err := os.RemoveAll(backupRoot); err != nil {
			return fmt.Errorf("remove bundle backup %q: %w", backupRoot, err)
		}
	}
	return nil
}

func extractTarGz(archivePath, destinationDir string) error {
	file, err := os.Open(archivePath) //nolint:gosec
	if err != nil {
		return fmt.Errorf("open archive %q: %w", archivePath, err)
	}
	defer func() { _ = file.Close() }()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open gzip archive %q: %w", archivePath, err)
	}
	defer func() { _ = gzipReader.Close() }()
	if err := extractTarStream(gzipReader, destinationDir); err != nil {
		return fmt.Errorf("extract tar archive %q: %w", archivePath, err)
	}
	return nil
}

func extractTarStream(reader io.Reader, destinationDir string) error {
	if err := os.MkdirAll(destinationDir, 0o750); err != nil {
		return fmt.Errorf("create tar destination %q: %w", destinationDir, err)
	}
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read tar stream: %w", err)
		}

		cleanTargetPath, err := tarEntryTargetPath(destinationDir, header.Name)
		if err != nil {
			return fmt.Errorf("tar entry %q escapes destination root: %w", header.Name, err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(cleanTargetPath, 0o750); err != nil {
				return fmt.Errorf("create tar directory %q: %w", cleanTargetPath, err)
			}
			if err := os.Chmod(cleanTargetPath, os.FileMode(header.Mode)); err != nil { //nolint:gosec
				return fmt.Errorf("chmod tar directory %q: %w", cleanTargetPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(cleanTargetPath), 0o750); err != nil {
				return fmt.Errorf("create parent directory for %q: %w", cleanTargetPath, err)
			}
			outFile, err := os.OpenFile(cleanTargetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode)) //nolint:gosec
			if err != nil {
				return fmt.Errorf("create tar file %q: %w", cleanTargetPath, err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil { //nolint:gosec
				_ = outFile.Close()
				return fmt.Errorf("write tar file %q: %w", cleanTargetPath, err)
			}
			if err := outFile.Close(); err != nil {
				return fmt.Errorf("close tar file %q: %w", cleanTargetPath, err)
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("refuse tar link entry %q", header.Name)
		case tar.TypeXHeader, tar.TypeXGlobalHeader, tar.TypeGNULongName, tar.TypeGNULongLink:
			continue
		default:
			return fmt.Errorf("unsupported tar entry type %d for %q", header.Typeflag, header.Name)
		}
	}
}

func tarEntryTargetPath(destinationDir, entryName string) (string, error) {
	cleanEntryName := filepath.Clean(filepath.FromSlash(entryName))
	if filepath.IsAbs(cleanEntryName) {
		return "", errors.New("absolute paths are not allowed")
	}
	if cleanEntryName == "." {
		return filepath.Clean(destinationDir), nil
	}
	if cleanEntryName == ".." || strings.HasPrefix(cleanEntryName, ".."+string(filepath.Separator)) {
		return "", errors.New("parent traversal is not allowed")
	}
	cleanDestinationDir := filepath.Clean(destinationDir)
	cleanTargetPath, err := securejoin.SecureJoin(cleanDestinationDir, cleanEntryName)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(cleanTargetPath, cleanDestinationDir+string(os.PathSeparator)) && cleanTargetPath != cleanDestinationDir {
		return "", errors.New("target path escapes destination root")
	}
	return cleanTargetPath, nil
}
