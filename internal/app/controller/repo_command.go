package controller

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"fmt"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type repoWriteResult struct {
	CommitID             string
	SyncStatus           string
	PushError            string
	LastSuccessfulPullAt string
}

type repoCommandServer struct {
	db                *store.DB
	cfg               *config.ControllerConfig
	availableNodeIDs  map[string]struct{}
	repoMu            *sync.Mutex
	pushCurrentBranch func(repoDir, remoteURL, branch, authUsername, authToken string) error
}

func (server *repoCommandServer) SyncRepo(ctx context.Context, _ *connect.Request[controllerv1.SyncRepoRequest]) (*connect.Response[controllerv1.SyncRepoResponse], error) {
	if !server.hasConfiguredRemote() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("repo remote sync is not configured"))
	}
	server.repoLock().Lock()
	defer server.repoLock().Unlock()

	if _, err := server.syncRepoLocked(ctx); err != nil {
		return nil, err
	}
	if err := server.refreshDeclaredServices(ctx); err != nil {
		return nil, err
	}
	headRevision, err := repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	branch, err := repo.CurrentBranch(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	syncState, err := server.repoSyncState(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&controllerv1.SyncRepoResponse{
		HeadRevision:         headRevision,
		Branch:               branch,
		SyncStatus:           syncState.SyncStatus,
		LastSyncError:        syncState.LastSyncError,
		LastSuccessfulPullAt: syncState.LastSuccessfulPullAt,
	}), nil
}

func (server *repoCommandServer) UpdateRepoFile(ctx context.Context, req *connect.Request[controllerv1.UpdateRepoFileRequest]) (*connect.Response[controllerv1.UpdateRepoFileResponse], error) {
	if req.Msg == nil || req.Msg.GetPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("path is required"))
	}
	if req.Msg.GetBaseRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("base_revision is required"))
	}
	result, err := server.runRepoWrite(ctx, req.Msg.GetBaseRevision(), []string{req.Msg.GetPath()}, func(baseSyncState store.RepoSyncState) (repoWriteResult, error) {
		return server.updateRepoFileTransaction(ctx, req.Msg.GetPath(), req.Msg.GetContent(), req.Msg.GetCommitMessage(), baseSyncState)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.UpdateRepoFileResponse{
		CommitId:             result.CommitID,
		SyncStatus:           result.SyncStatus,
		PushError:            result.PushError,
		LastSuccessfulPullAt: result.LastSuccessfulPullAt,
	}), nil
}

func (server *repoCommandServer) CreateRepoDirectory(ctx context.Context, req *connect.Request[controllerv1.CreateRepoDirectoryRequest]) (*connect.Response[controllerv1.CreateRepoDirectoryResponse], error) {
	if req.Msg == nil || req.Msg.GetPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("path is required"))
	}
	if req.Msg.GetBaseRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("base_revision is required"))
	}
	result, err := server.runRepoWrite(ctx, req.Msg.GetBaseRevision(), []string{req.Msg.GetPath()}, func(baseSyncState store.RepoSyncState) (repoWriteResult, error) {
		return server.createRepoDirectoryTransaction(ctx, req.Msg.GetPath(), req.Msg.GetCommitMessage(), baseSyncState)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.CreateRepoDirectoryResponse{
		CommitId:             result.CommitID,
		SyncStatus:           result.SyncStatus,
		PushError:            result.PushError,
		LastSuccessfulPullAt: result.LastSuccessfulPullAt,
	}), nil
}

func (server *repoCommandServer) MoveRepoPath(ctx context.Context, req *connect.Request[controllerv1.MoveRepoPathRequest]) (*connect.Response[controllerv1.MoveRepoPathResponse], error) {
	if req.Msg == nil || req.Msg.GetSourcePath() == "" || req.Msg.GetDestinationPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source_path and destination_path are required"))
	}
	if req.Msg.GetBaseRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("base_revision is required"))
	}
	result, err := server.runRepoWrite(ctx, req.Msg.GetBaseRevision(), []string{req.Msg.GetSourcePath(), req.Msg.GetDestinationPath()}, func(baseSyncState store.RepoSyncState) (repoWriteResult, error) {
		return server.moveRepoPathTransaction(ctx, req.Msg.GetSourcePath(), req.Msg.GetDestinationPath(), req.Msg.GetCommitMessage(), baseSyncState)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.MoveRepoPathResponse{
		CommitId:             result.CommitID,
		SyncStatus:           result.SyncStatus,
		PushError:            result.PushError,
		LastSuccessfulPullAt: result.LastSuccessfulPullAt,
	}), nil
}

func (server *repoCommandServer) DeleteRepoPath(ctx context.Context, req *connect.Request[controllerv1.DeleteRepoPathRequest]) (*connect.Response[controllerv1.DeleteRepoPathResponse], error) {
	if req.Msg == nil || req.Msg.GetPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("path is required"))
	}
	if req.Msg.GetBaseRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("base_revision is required"))
	}
	result, err := server.runRepoWrite(ctx, req.Msg.GetBaseRevision(), []string{req.Msg.GetPath()}, func(baseSyncState store.RepoSyncState) (repoWriteResult, error) {
		return server.deleteRepoPathTransaction(ctx, req.Msg.GetPath(), req.Msg.GetCommitMessage(), baseSyncState)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.DeleteRepoPathResponse{
		CommitId:             result.CommitID,
		SyncStatus:           result.SyncStatus,
		PushError:            result.PushError,
		LastSuccessfulPullAt: result.LastSuccessfulPullAt,
	}), nil
}

func (server *repoCommandServer) repoLock() *sync.Mutex {
	if server.repoMu == nil {
		server.repoMu = &sync.Mutex{}
	}
	return server.repoMu
}

func (server *repoCommandServer) runRepoWrite(ctx context.Context, baseRevision string, relativePaths []string, run func(baseSyncState store.RepoSyncState) (repoWriteResult, error)) (repoWriteResult, error) {
	server.repoLock().Lock()
	defer server.repoLock().Unlock()

	baseSyncState, err := server.prepareRepoWritePaths(ctx, baseRevision, relativePaths)
	if err != nil {
		return repoWriteResult{}, err
	}
	return run(baseSyncState)
}

func (server *repoCommandServer) hasConfiguredRemote() bool {
	return server.cfg != nil && server.cfg.Git != nil && strings.TrimSpace(server.cfg.Git.RemoteURL) != ""
}

func (server *repoCommandServer) repoSyncState(ctx context.Context) (store.RepoSyncState, error) {
	if !server.hasConfiguredRemote() {
		return store.RepoSyncState{SyncStatus: store.RepoSyncStatusLocalOnly}, nil
	}
	if server.db == nil {
		return store.RepoSyncState{SyncStatus: store.RepoSyncStatusUnknown}, nil
	}
	state, err := server.db.GetRepoSyncState(ctx)
	if err != nil {
		return store.RepoSyncState{}, err
	}
	if state.SyncStatus == "" {
		state.SyncStatus = store.RepoSyncStatusUnknown
	}
	return state, nil
}

func (server *repoCommandServer) configuredRemoteBranch() (string, error) {
	if server.cfg != nil && server.cfg.Git != nil && strings.TrimSpace(server.cfg.Git.Branch) != "" {
		return strings.TrimSpace(server.cfg.Git.Branch), nil
	}
	branch, err := repo.CurrentBranch(server.cfg.RepoDir)
	if err != nil {
		return "", err
	}
	if branch == "" {
		return "", fmt.Errorf("cannot determine repo branch for remote sync")
	}
	return branch, nil
}

func (server *repoCommandServer) configuredGitAuthToken() (string, error) {
	if server.cfg == nil || server.cfg.Git == nil || server.cfg.Git.Auth == nil {
		return "", nil
	}
	return strings.TrimSpace(server.cfg.Git.Auth.Token), nil
}

func (server *repoCommandServer) configuredGitAuthUsername() string {
	if server.cfg == nil || server.cfg.Git == nil || server.cfg.Git.Auth == nil {
		return ""
	}
	return strings.TrimSpace(server.cfg.Git.Auth.Username)
}

func (server *repoCommandServer) persistRepoSyncState(ctx context.Context, state store.RepoSyncState) error {
	if server.db == nil {
		return nil
	}
	return server.db.UpsertRepoSyncState(ctx, state)
}

func (server *repoCommandServer) ensureCleanWorktree() error {
	cleanWorktree, err := repo.IsCleanWorkingTree(server.cfg.RepoDir)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if !cleanWorktree {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("repo working tree is not clean"))
	}
	return nil
}

func (server *repoCommandServer) syncRepoLocked(ctx context.Context) (store.RepoSyncState, error) {
	if !server.hasConfiguredRemote() {
		return store.RepoSyncState{SyncStatus: store.RepoSyncStatusLocalOnly}, connect.NewError(connect.CodeFailedPrecondition, errors.New("repo remote sync is not configured"))
	}
	if err := server.ensureCleanWorktree(); err != nil {
		return store.RepoSyncState{}, err
	}
	branch, err := server.configuredRemoteBranch()
	if err != nil {
		return store.RepoSyncState{}, connect.NewError(connect.CodeInternal, err)
	}
	authToken, err := server.configuredGitAuthToken()
	if err != nil {
		return store.RepoSyncState{}, connect.NewError(connect.CodeInternal, err)
	}
	previousState, err := server.repoSyncState(ctx)
	if err != nil {
		return store.RepoSyncState{}, connect.NewError(connect.CodeInternal, err)
	}
	pulledAt := time.Now().UTC().Format(time.RFC3339)
	if err := repo.FetchAndFastForward(server.cfg.RepoDir, strings.TrimSpace(server.cfg.Git.RemoteURL), branch, server.configuredGitAuthUsername(), authToken); err != nil {
		state := store.RepoSyncState{
			SyncStatus:           store.RepoSyncStatusPullFailed,
			LastSyncError:        err.Error(),
			LastSuccessfulPullAt: previousState.LastSuccessfulPullAt,
		}
		if persistErr := server.persistRepoSyncState(ctx, state); persistErr != nil {
			return store.RepoSyncState{}, connect.NewError(connect.CodeInternal, persistErr)
		}
		return state, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	state := store.RepoSyncState{
		SyncStatus:           store.RepoSyncStatusSynced,
		LastSyncError:        "",
		LastSuccessfulPullAt: pulledAt,
	}
	if err := server.persistRepoSyncState(ctx, state); err != nil {
		return store.RepoSyncState{}, connect.NewError(connect.CodeInternal, err)
	}
	return state, nil
}

func (server *repoCommandServer) prepareRepoWritePaths(ctx context.Context, baseRevision string, relativePaths []string) (store.RepoSyncState, error) {
	if err := server.syncRepoBeforeWrite(ctx); err != nil {
		return store.RepoSyncState{}, err
	}
	if err := server.verifyRepoWriteBaseRevision(baseRevision); err != nil {
		return store.RepoSyncState{}, err
	}
	if err := server.verifyRepoWriteAllowed(ctx, relativePaths...); err != nil {
		return store.RepoSyncState{}, err
	}
	return server.repoSyncState(ctx)
}

func (server *repoCommandServer) syncRepoBeforeWrite(ctx context.Context) error {
	if !server.hasConfiguredRemote() {
		return nil
	}
	_, err := server.syncRepoLocked(ctx)
	return err
}

func (server *repoCommandServer) verifyRepoWriteBaseRevision(baseRevision string) error {
	currentRevision, err := repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if currentRevision != baseRevision {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("base_revision %q does not match current HEAD %q", baseRevision, currentRevision))
	}
	return nil
}

func (server *repoCommandServer) verifyRepoWriteAllowed(ctx context.Context, relativePaths ...string) error {
	if err := server.ensureCleanWorktree(); err != nil {
		return err
	}
	return server.ensureRepoPathsUnlocked(ctx, relativePaths...)
}

func (server *repoCommandServer) refreshDeclaredServices(ctx context.Context) error {
	if server.db == nil {
		return nil
	}
	services, err := repo.DiscoverServices(server.cfg.RepoDir, server.availableNodeIDs)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	declaredServices := make(map[string][]string, len(services))
	for _, service := range services {
		declaredServices[service.Name] = append([]string(nil), service.TargetNodes...)
	}
	if err := server.db.SyncDeclaredServices(ctx, declaredServices); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	return nil
}

func (server *repoCommandServer) ensureRepoPathsUnlocked(ctx context.Context, relativePaths ...string) error {
	if server.db == nil {
		return nil
	}
	services, err := repo.DiscoverServices(server.cfg.RepoDir, server.availableNodeIDs)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	for _, relativePath := range relativePaths {
		cleanPath := filepath.ToSlash(filepath.Clean(relativePath))
		for _, service := range services {
			serviceDir, err := filepath.Rel(server.cfg.RepoDir, service.Directory)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("resolve service directory: %w", err))
			}
			serviceDir = filepath.ToSlash(filepath.Clean(serviceDir))
			if !pathHitsServiceDir(cleanPath, serviceDir) {
				continue
			}
			active, err := server.db.HasActiveServiceTask(ctx, service.Name)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			if active {
				return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q has an active task", service.Name))
			}
		}
	}
	return nil
}

func pathHitsServiceDir(targetPath, serviceDir string) bool {
	if targetPath == serviceDir {
		return true
	}
	return strings.HasPrefix(targetPath, serviceDir+"/")
}

func (server *repoCommandServer) updateRepoFileTransaction(ctx context.Context, relativePath, content, commitMessage string, baseSyncState store.RepoSyncState) (_ repoWriteResult, retErr error) {
	previous, readErr := repo.ReadFile(server.cfg.RepoDir, relativePath)
	fileExisted := readErr == nil
	committed := false
	if readErr != nil && !errors.Is(readErr, repo.ErrRepoPathNotFound) {
		switch {
		case errors.Is(readErr, repo.ErrRepoPathInvalid), errors.Is(readErr, repo.ErrRepoPathProtected):
			return repoWriteResult{}, connect.NewError(connect.CodeInvalidArgument, readErr)
		case errors.Is(readErr, repo.ErrRepoPathNotFile):
			return repoWriteResult{}, connect.NewError(connect.CodeFailedPrecondition, readErr)
		default:
			return repoWriteResult{}, connect.NewError(connect.CodeInternal, readErr)
		}
	}
	writtenPath, err := repo.WriteFile(server.cfg.RepoDir, relativePath, content)
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrRepoPathInvalid), errors.Is(err, repo.ErrRepoPathProtected):
			return repoWriteResult{}, connect.NewError(connect.CodeInvalidArgument, err)
		default:
			return repoWriteResult{}, connect.NewError(connect.CodeInternal, err)
		}
	}
	defer func() {
		if retErr == nil || committed {
			return
		}
		if fileExisted {
			_, _ = repo.WriteFile(server.cfg.RepoDir, writtenPath, previous.Content)
		} else {
			absolutePath := filepath.Join(server.cfg.RepoDir, filepath.FromSlash(writtenPath))
			_ = os.Remove(absolutePath)
		}
	}()
	authorName := ""
	authorEmail := ""
	if server.cfg.Git != nil {
		authorName = server.cfg.Git.AuthorName
		authorEmail = server.cfg.Git.AuthorEmail
	}
	commitID, err := repo.CommitPath(server.cfg.RepoDir, writtenPath, commitMessage, authorName, authorEmail)
	if err != nil {
		if errors.Is(err, repo.ErrNoGitChanges) {
			return repoWriteResult{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("repo file content did not change"))
		}
		return repoWriteResult{}, connect.NewError(connect.CodeInternal, err)
	}
	committed = true
	return server.finalizeRepoWrite(ctx, commitID, baseSyncState)
}

func (server *repoCommandServer) createRepoDirectoryTransaction(ctx context.Context, relativePath, commitMessage string, baseSyncState store.RepoSyncState) (_ repoWriteResult, retErr error) {
	snapshot, err := repo.CapturePath(server.cfg.RepoDir, relativePath)
	if err != nil {
		return repoWriteResult{}, mapRepoMutationError(err)
	}
	defer func() { _ = repo.CleanupPathSnapshot(snapshot) }()
	committed := false
	createdPath, err := repo.CreateDirectory(server.cfg.RepoDir, relativePath)
	if err != nil {
		return repoWriteResult{}, mapRepoMutationError(err)
	}
	defer func() {
		if retErr == nil || committed {
			return
		}
		_ = repo.RestorePath(server.cfg.RepoDir, snapshot)
	}()
	message := commitMessage
	if message == "" {
		message = defaultRepoCommitMessage("add", createdPath)
	}
	commitID, err := server.commitRepoPaths(createdPath, []string{createdPath}, message)
	if err != nil {
		return repoWriteResult{}, err
	}
	committed = true
	return server.finalizeRepoWrite(ctx, commitID, baseSyncState)
}

func (server *repoCommandServer) moveRepoPathTransaction(ctx context.Context, sourcePath, destinationPath, commitMessage string, baseSyncState store.RepoSyncState) (_ repoWriteResult, retErr error) {
	sourceSnapshot, err := repo.CapturePath(server.cfg.RepoDir, sourcePath)
	if err != nil {
		return repoWriteResult{}, mapRepoMutationError(err)
	}
	defer func() { _ = repo.CleanupPathSnapshot(sourceSnapshot) }()
	destinationSnapshot, err := repo.CapturePath(server.cfg.RepoDir, destinationPath)
	if err != nil {
		return repoWriteResult{}, mapRepoMutationError(err)
	}
	defer func() { _ = repo.CleanupPathSnapshot(destinationSnapshot) }()
	committed := false
	movedSource, movedDestination, err := repo.MovePath(server.cfg.RepoDir, sourcePath, destinationPath)
	if err != nil {
		return repoWriteResult{}, mapRepoMutationError(err)
	}
	defer func() {
		if retErr == nil || committed {
			return
		}
		_ = repo.RestorePath(server.cfg.RepoDir, destinationSnapshot)
		_ = repo.RestorePath(server.cfg.RepoDir, sourceSnapshot)
	}()
	message := commitMessage
	if message == "" {
		message = fmt.Sprintf("move %s to %s", movedSource, movedDestination)
	}
	commitID, err := server.commitRepoPaths(movedDestination, []string{movedSource, movedDestination}, message)
	if err != nil {
		return repoWriteResult{}, err
	}
	committed = true
	return server.finalizeRepoWrite(ctx, commitID, baseSyncState)
}

func (server *repoCommandServer) deleteRepoPathTransaction(ctx context.Context, relativePath, commitMessage string, baseSyncState store.RepoSyncState) (_ repoWriteResult, retErr error) {
	snapshot, err := repo.CapturePath(server.cfg.RepoDir, relativePath)
	if err != nil {
		return repoWriteResult{}, mapRepoMutationError(err)
	}
	defer func() { _ = repo.CleanupPathSnapshot(snapshot) }()
	committed := false
	deletedPath, err := repo.DeletePath(server.cfg.RepoDir, relativePath)
	if err != nil {
		return repoWriteResult{}, mapRepoMutationError(err)
	}
	defer func() {
		if retErr == nil || committed {
			return
		}
		_ = repo.RestorePath(server.cfg.RepoDir, snapshot)
	}()
	message := commitMessage
	if message == "" {
		message = defaultRepoCommitMessage("remove", deletedPath)
	}
	commitID, err := server.commitRepoPaths(deletedPath, []string{deletedPath}, message)
	if err != nil {
		return repoWriteResult{}, err
	}
	committed = true
	return server.finalizeRepoWrite(ctx, commitID, baseSyncState)
}

func (server *repoCommandServer) commitRepoPaths(primaryPath string, relativePaths []string, commitMessage string) (string, error) {
	authorName := ""
	authorEmail := ""
	if server.cfg.Git != nil {
		authorName = server.cfg.Git.AuthorName
		authorEmail = server.cfg.Git.AuthorEmail
	}
	commitID, err := repo.CommitPaths(server.cfg.RepoDir, relativePaths, commitMessage, authorName, authorEmail)
	if err != nil {
		if errors.Is(err, repo.ErrNoGitChanges) {
			return "", connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("repo path %q did not change", primaryPath))
		}
		return "", connect.NewError(connect.CodeInternal, err)
	}
	return commitID, nil
}

func (server *repoCommandServer) finalizeRepoWrite(ctx context.Context, commitID string, baseSyncState store.RepoSyncState) (repoWriteResult, error) {
	result, err := server.finalizeRepoGitState(ctx, commitID, baseSyncState)
	if err != nil {
		return repoWriteResult{}, err
	}
	if err := server.refreshDeclaredServices(ctx); err != nil {
		return repoWriteResult{}, err
	}
	return result, nil
}

func (server *repoCommandServer) finalizeRepoGitState(ctx context.Context, commitID string, baseSyncState store.RepoSyncState) (repoWriteResult, error) {
	result := repoWriteResult{CommitID: commitID, SyncStatus: baseSyncState.SyncStatus, LastSuccessfulPullAt: baseSyncState.LastSuccessfulPullAt}
	if !server.hasConfiguredRemote() {
		result.SyncStatus = store.RepoSyncStatusLocalOnly
		return result, nil
	}
	branch, err := server.configuredRemoteBranch()
	if err != nil {
		return repoWriteResult{}, connect.NewError(connect.CodeInternal, err)
	}
	authToken, err := server.configuredGitAuthToken()
	if err != nil {
		return repoWriteResult{}, connect.NewError(connect.CodeInternal, err)
	}
	pushCurrentBranch := server.pushCurrentBranch
	if pushCurrentBranch == nil {
		pushCurrentBranch = repo.PushCurrentBranch
	}
	if err := pushCurrentBranch(server.cfg.RepoDir, strings.TrimSpace(server.cfg.Git.RemoteURL), branch, server.configuredGitAuthUsername(), authToken); err != nil {
		state := store.RepoSyncState{
			SyncStatus:           store.RepoSyncStatusPushFailed,
			LastSyncError:        err.Error(),
			LastSuccessfulPullAt: baseSyncState.LastSuccessfulPullAt,
		}
		if persistErr := server.persistRepoSyncState(ctx, state); persistErr != nil {
			return repoWriteResult{}, connect.NewError(connect.CodeInternal, persistErr)
		}
		result.SyncStatus = state.SyncStatus
		result.PushError = state.LastSyncError
		result.LastSuccessfulPullAt = state.LastSuccessfulPullAt
		return result, nil
	}
	state := store.RepoSyncState{
		SyncStatus:           store.RepoSyncStatusSynced,
		LastSyncError:        "",
		LastSuccessfulPullAt: baseSyncState.LastSuccessfulPullAt,
	}
	if err := server.persistRepoSyncState(ctx, state); err != nil {
		return repoWriteResult{}, connect.NewError(connect.CodeInternal, err)
	}
	result.SyncStatus = state.SyncStatus
	result.LastSuccessfulPullAt = state.LastSuccessfulPullAt
	return result, nil
}

func mapRepoMutationError(err error) error {
	switch {
	case errors.Is(err, repo.ErrRepoPathInvalid), errors.Is(err, repo.ErrRepoPathProtected):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, repo.ErrRepoPathNotFound), errors.Is(err, repo.ErrRepoPathAlreadyExists), errors.Is(err, repo.ErrRepoPathNotFile), errors.Is(err, repo.ErrRepoPathNotDirectory):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func defaultRepoCommitMessage(action, relativePath string) string {
	return fmt.Sprintf("%s %s", action, relativePath)
}
