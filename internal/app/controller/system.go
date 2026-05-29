package controller

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"forgejo.alexma.top/alexma233/composia/internal/version"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type systemServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	reload           func(context.Context) error
}

func (server *systemServer) GetSystemStatus(ctx context.Context, _ *connect.Request[controllerv1.GetSystemStatusRequest]) (*connect.Response[controllerv1.GetSystemStatusResponse], error) {
	configured, online, err := server.db.NodeCounts(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	response := &controllerv1.GetSystemStatusResponse{
		Version:             version.Value,
		Now:                 timestamppb.Now(),
		ConfiguredNodeCount: configured,
		OnlineNodeCount:     online,
	}
	return connect.NewResponse(response), nil
}

func (server *systemServer) ReloadControllerConfig(ctx context.Context, _ *connect.Request[controllerv1.ReloadControllerConfigRequest]) (*connect.Response[controllerv1.ReloadControllerConfigResponse], error) {
	if server.reload == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("controller reload is not available"))
	}
	if err := server.reload(ctx); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewResponse(&controllerv1.ReloadControllerConfigResponse{Accepted: true}), nil
}

func (server *systemServer) GetCurrentConfig(ctx context.Context, _ *connect.Request[controllerv1.GetCurrentConfigRequest]) (*connect.Response[controllerv1.GetCurrentConfigResponse], error) {
	response := &controllerv1.GetCurrentConfigResponse{
		ListenAddr: server.cfg.ListenAddr,
	}

	if server.cfg.Git != nil {
		response.Git = &controllerv1.GitConfigSummary{
			RemoteUrl:    server.cfg.Git.RemoteURL,
			Branch:       server.cfg.Git.Branch,
			PullInterval: server.cfg.Git.PullInterval,
			HasAuth:      server.cfg.Git.Auth != nil && strings.TrimSpace(server.cfg.Git.Auth.Token) != "",
			AuthorName:   server.cfg.Git.AuthorName,
			AuthorEmail:  server.cfg.Git.AuthorEmail,
		}
	}

	response.Nodes = make([]*controllerv1.NodeConfigSummary, 0, len(server.cfg.Nodes))
	for _, node := range server.cfg.Nodes {
		enabled := true
		if node.Enabled != nil {
			enabled = *node.Enabled
		}
		response.Nodes = append(response.Nodes, &controllerv1.NodeConfigSummary{
			Id:          node.ID,
			DisplayName: node.DisplayName,
			Enabled:     enabled,
			PublicIpv4:  node.PublicIPv4,
			PublicIpv6:  node.PublicIPv6,
		})
	}

	response.AccessTokens = make([]*controllerv1.AccessTokenSummary, 0, len(server.cfg.AccessTokens))
	for _, token := range server.cfg.AccessTokens {
		enabled := true
		if token.Enabled != nil {
			enabled = *token.Enabled
		}
		response.AccessTokens = append(response.AccessTokens, &controllerv1.AccessTokenSummary{
			Name:    token.Name,
			Enabled: enabled,
			Comment: token.Comment,
		})
	}

	if server.cfg.DNS != nil && server.cfg.DNS.Cloudflare != nil {
		response.Dns = &controllerv1.DNSConfigSummary{
			HasCloudflare: strings.TrimSpace(server.cfg.DNS.Cloudflare.APIToken) != "",
		}
	}

	if _, err := repo.FindRusticInfraService(server.cfg.RepoDir, server.availableNodeIDs); err == nil {
		response.Backup = &controllerv1.BackupConfigSummary{
			HasRustic: true,
		}
	}

	if server.cfg.Secrets != nil {
		response.Secrets = &controllerv1.SecretsConfigSummary{
			Provider:     server.cfg.Secrets.Provider,
			HasIdentity:  server.cfg.Secrets.IdentityFile != "",
			HasRecipient: server.cfg.Secrets.RecipientFile != "",
		}
	}

	return connect.NewResponse(response), nil
}
