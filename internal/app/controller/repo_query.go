package controller

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"sync"
)

type repoQueryServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	repoMu           *sync.Mutex
}

func (server *repoQueryServer) GetRepoHead(ctx context.Context, _ *connect.Request[controllerv1.GetRepoHeadRequest]) (*connect.Response[controllerv1.GetRepoHeadResponse], error) {
	headRevision, err := repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	branch, err := repo.CurrentBranch(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	cleanWorktree, err := repo.IsCleanWorkingTree(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	syncState, err := server.repoSyncState(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.GetRepoHeadResponse{
		HeadRevision:         headRevision,
		Branch:               branch,
		HasRemote:            server.hasConfiguredRemote(),
		CleanWorktree:        cleanWorktree,
		SyncStatus:           syncState.SyncStatus,
		LastSyncError:        syncState.LastSyncError,
		LastSuccessfulPullAt: syncState.LastSuccessfulPullAt,
	}
	return connect.NewResponse(response), nil
}

func (server *repoQueryServer) ListRepoFiles(_ context.Context, req *connect.Request[controllerv1.ListRepoFilesRequest]) (*connect.Response[controllerv1.ListRepoFilesResponse], error) {
	if req.Msg == nil {
		req.Msg = &controllerv1.ListRepoFilesRequest{}
	}
	entries, err := repo.ListFiles(server.cfg.RepoDir, req.Msg.GetPath(), req.Msg.GetRecursive())
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrRepoPathInvalid):
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		case errors.Is(err, repo.ErrRepoPathNotFound), errors.Is(err, repo.ErrRepoPathNotDirectory):
			return nil, connect.NewError(connect.CodeNotFound, err)
		default:
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	response := &controllerv1.ListRepoFilesResponse{Entries: make([]*controllerv1.RepoFileEntry, 0, len(entries))}
	for _, entry := range entries {
		response.Entries = append(response.Entries, &controllerv1.RepoFileEntry{
			Path:  entry.Path,
			Name:  entry.Name,
			IsDir: entry.IsDir,
			Size:  entry.Size,
		})
	}
	return connect.NewResponse(response), nil
}

func (server *repoQueryServer) GetRepoFile(_ context.Context, req *connect.Request[controllerv1.GetRepoFileRequest]) (*connect.Response[controllerv1.GetRepoFileResponse], error) {
	if req.Msg == nil || req.Msg.GetPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("path is required"))
	}
	file, err := repo.ReadFile(server.cfg.RepoDir, req.Msg.GetPath())
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrRepoPathInvalid):
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		case errors.Is(err, repo.ErrRepoPathNotFound), errors.Is(err, repo.ErrRepoPathNotFile):
			return nil, connect.NewError(connect.CodeNotFound, err)
		default:
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	response := &controllerv1.GetRepoFileResponse{
		Path:    file.Path,
		Content: file.Content,
		Size:    file.Size,
	}
	return connect.NewResponse(response), nil
}

func (server *repoQueryServer) ListRepoCommits(_ context.Context, req *connect.Request[controllerv1.ListRepoCommitsRequest]) (*connect.Response[controllerv1.ListRepoCommitsResponse], error) {
	if req.Msg == nil {
		req.Msg = &controllerv1.ListRepoCommitsRequest{}
	}
	commits, nextCursor, err := repo.ListCommits(server.cfg.RepoDir, req.Msg.GetCursor(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.ListRepoCommitsResponse{
		Commits:    make([]*controllerv1.RepoCommitSummary, 0, len(commits)),
		NextCursor: nextCursor,
	}
	for _, commit := range commits {
		response.Commits = append(response.Commits, &controllerv1.RepoCommitSummary{
			CommitId:    commit.CommitID,
			Subject:     commit.Subject,
			CommittedAt: commit.CommittedAt,
		})
	}
	return connect.NewResponse(response), nil
}

func (server *repoQueryServer) ValidateRepo(_ context.Context, _ *connect.Request[controllerv1.ValidateRepoRequest]) (*connect.Response[controllerv1.ValidateRepoResponse], error) {
	availableNodeIDs := make(map[string]struct{}, len(server.cfg.Nodes))
	for _, node := range server.cfg.Nodes {
		availableNodeIDs[node.ID] = struct{}{}
	}
	validationErrors := repo.ValidateRepo(server.cfg.RepoDir, availableNodeIDs)
	response := &controllerv1.ValidateRepoResponse{Errors: make([]*controllerv1.RepoValidationError, 0, len(validationErrors))}
	for _, validationError := range validationErrors {
		response.Errors = append(response.Errors, &controllerv1.RepoValidationError{
			Path:    validationError.Path,
			Line:    validationError.Line,
			Message: validationError.Message,
		})
	}
	return connect.NewResponse(response), nil
}

func (server *repoQueryServer) hasConfiguredRemote() bool {
	return (&repoCommandServer{cfg: server.cfg}).hasConfiguredRemote()
}

func (server *repoQueryServer) repoSyncState(ctx context.Context) (store.RepoSyncState, error) {
	return (&repoCommandServer{db: server.db, cfg: server.cfg}).repoSyncState(ctx)
}
