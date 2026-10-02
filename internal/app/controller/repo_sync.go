package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"

	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func (server *repoCommandServer) syncLocalFirstRepo(ctx context.Context) (previousRevision, pulledRevision string, retErr error) {
	// Fetch updates FETCH_HEAD, so manual and periodic sync must share this lock.
	server.syncMu.Lock()
	defer server.syncMu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	branch, err := server.configuredRemoteBranch()
	if err != nil {
		return "", "", err
	}
	authToken, err := server.configuredGitAuthToken()
	if err != nil {
		return "", "", err
	}
	remoteURL := strings.TrimSpace(server.cfg.Git.RemoteURL)
	fetchedRevision, err := repo.FetchRevision(ctx, server.cfg.RepoDir, remoteURL, branch, server.configuredGitAuthUsername(), authToken)
	if err != nil {
		return "", "", server.recordRepoSyncFailure(ctx, store.RepoSyncStatusPullFailed, "", err)
	}
	pulledAt := time.Now().UTC().Format(time.RFC3339)
	previousRevision, pulledRevision, err = server.applyFetchedRevision(ctx, fetchedRevision)
	if err != nil {
		return previousRevision, pulledRevision, server.recordRepoSyncFailure(ctx, store.RepoSyncStatusPullFailed, "", err)
	}
	// Pin the pushed commit: local writes may advance HEAD while the network stalls.
	if err := repo.PushRevision(ctx, server.cfg.RepoDir, remoteURL, branch, pulledRevision, server.configuredGitAuthUsername(), authToken); err != nil {
		return previousRevision, pulledRevision, server.recordRepoSyncFailure(ctx, store.RepoSyncStatusPushFailed, pulledAt, err)
	}
	server.repoLock().Lock()
	defer server.repoLock().Unlock()
	currentRevision, err := repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		return previousRevision, pulledRevision, err
	}
	state := store.RepoSyncState{SyncStatus: store.RepoSyncStatusSynced, LastSuccessfulPullAt: pulledAt}
	if currentRevision != pulledRevision {
		state.SyncStatus = store.RepoSyncStatusPendingSync
	}
	return previousRevision, pulledRevision, server.persistRepoSyncState(ctx, state)
}

func (server *repoCommandServer) applyFetchedRevision(ctx context.Context, revision string) (previousRevision, pulledRevision string, retErr error) {
	server.repoLock().Lock()
	defer server.repoLock().Unlock()
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	if err := server.ensureCleanWorktree(); err != nil {
		return "", "", err
	}
	previousRevision, err := repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		return "", "", err
	}
	if err := repo.FastForwardRevision(server.cfg.RepoDir, revision); err != nil {
		return previousRevision, "", connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("repo sync requires manual reconciliation: %w", err))
	}
	pulledRevision, err = repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		return previousRevision, "", err
	}
	return previousRevision, pulledRevision, server.refreshDeclaredServices(ctx)
}

func (server *repoCommandServer) recordRepoSyncFailure(ctx context.Context, status, pulledAt string, syncErr error) error {
	server.repoLock().Lock()
	defer server.repoLock().Unlock()
	state, err := server.repoSyncState(ctx)
	if err != nil {
		return err
	}
	state.SyncStatus = status
	state.LastSyncError = syncErr.Error()
	if pulledAt != "" {
		state.LastSuccessfulPullAt = pulledAt
	}
	if err := server.persistRepoSyncState(ctx, state); err != nil {
		return err
	}
	return syncErr
}
