package controller

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"connectrpc.com/connect"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	secretutil "forgejo.alexma.top/alexma233/composia/internal/platform/secret"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

type secretServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	repoMu           *sync.Mutex
}

func (server *secretServer) GetSecret(ctx context.Context, req *connect.Request[controllerv1.GetSecretRequest]) (*connect.Response[controllerv1.GetSecretResponse], error) {
	if req.Msg == nil || req.Msg.GetFilePath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("file_path is required"))
	}
	if req.Msg.GetServiceDir() == "" && req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_dir or service_name is required"))
	}
	if server.cfg.Secrets == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("controller secrets are not configured"))
	}
	displayName, filePath, err := server.resolveSecretFilePath(req.Msg.GetServiceDir(), req.Msg.GetServiceName(), req.Msg.GetFilePath())
	if err != nil {
		return nil, err
	}
	secretFile, err := repo.ReadFile(server.cfg.RepoDir, filePath)
	if err != nil {
		if errors.Is(err, repo.ErrRepoPathNotFound) {
			return connect.NewResponse(&controllerv1.GetSecretResponse{ServiceName: displayName, FilePath: req.Msg.GetFilePath(), Content: ""}), nil
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	plaintext, err := secretutil.Decrypt([]byte(secretFile.Content), server.cfg.Secrets)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&controllerv1.GetSecretResponse{ServiceName: displayName, FilePath: req.Msg.GetFilePath(), Content: plaintext}), nil
}

func (server *secretServer) UpdateSecret(ctx context.Context, req *connect.Request[controllerv1.UpdateSecretRequest]) (*connect.Response[controllerv1.RepoWriteResult], error) {
	if req.Msg == nil || req.Msg.GetFilePath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("file_path is required"))
	}
	if req.Msg.GetServiceDir() == "" && req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_dir or service_name is required"))
	}
	if req.Msg.GetBaseRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("base_revision is required"))
	}
	if server.cfg.Secrets == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("controller secrets are not configured"))
	}
	displayName, filePath, err := server.resolveSecretFilePath(req.Msg.GetServiceDir(), req.Msg.GetServiceName(), req.Msg.GetFilePath())
	if err != nil {
		return nil, err
	}
	ciphertext, err := secretutil.Encrypt(req.Msg.GetContent(), server.cfg.Secrets)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	repoSrv := &repoCommandServer{db: server.db, cfg: server.cfg, availableNodeIDs: server.availableNodeIDs, repoMu: server.repoMu}
	commitMessage := req.Msg.GetCommitMessage()
	if commitMessage == "" {
		commitMessage = fmt.Sprintf("update encrypted file %s for %s", req.Msg.GetFilePath(), displayName)
	}
	result, err := repoSrv.runRepoWrite(ctx, req.Msg.GetBaseRevision(), []string{filePath}, func(baseSyncState store.RepoSyncState) (repoWriteResult, error) {
		return repoSrv.updateRepoFileTransaction(ctx, filePath, string(ciphertext), commitMessage, baseSyncState)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(repoWriteResultMessage(result)), nil
}

func (server *secretServer) resolveSecretFilePath(serviceDir, serviceName, filePath string) (string, string, error) {
	cleanPath := filepath.ToSlash(filepath.Clean(filePath))
	if strings.HasPrefix(cleanPath, "../") || strings.Contains(cleanPath, "/../") {
		return "", "", connect.NewError(connect.CodeInvalidArgument, errors.New("file_path must not escape service directory"))
	}
	if filepath.IsAbs(cleanPath) {
		return "", "", connect.NewError(connect.CodeInvalidArgument, errors.New("file_path must be relative"))
	}
	if serviceDir != "" {
		normalizedDir := filepath.ToSlash(filepath.Clean(serviceDir))
		if strings.HasPrefix(normalizedDir, "../") || strings.Contains(normalizedDir, "/../") {
			return "", "", connect.NewError(connect.CodeInvalidArgument, errors.New("service_dir must not escape repo root"))
		}
		if filepath.IsAbs(normalizedDir) {
			return "", "", connect.NewError(connect.CodeInvalidArgument, errors.New("service_dir must be relative"))
		}
		fullPath := filepath.ToSlash(filepath.Join(normalizedDir, cleanPath))
		displayName := filepath.Base(normalizedDir)
		return displayName, fullPath, nil
	}
	service, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, serviceName)
	if err != nil {
		return "", "", connect.NewError(connect.CodeNotFound, err)
	}
	resolvedDir, err := filepath.Rel(server.cfg.RepoDir, service.Directory)
	if err != nil {
		return "", "", connect.NewError(connect.CodeInternal, fmt.Errorf("resolve service directory: %w", err))
	}
	fullPath := filepath.ToSlash(filepath.Join(resolvedDir, cleanPath))
	return service.Name, fullPath, nil
}
