package controller

import (
	"context"
	"slices"
	"strings"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

const (
	reasonMissingBackupIntegration  = "missing_backup_integration"
	reasonMissingBackupDefinition   = "missing_backup_definition"
	reasonMissingRestoreDefinition  = "missing_restore_definition"
	reasonMissingMigrateDefinition  = "missing_migrate_definition"
	reasonMissingDNSIntegration     = "missing_dns_integration"
	reasonMissingSecretsConfig      = "missing_secrets_config"
	reasonMissingCaddyInfra         = "missing_caddy_infra"
	reasonMissingServiceMeta        = "missing_service_meta"
	reasonServiceNotDeclared        = "service_not_declared"
	reasonServiceDNSNotDeclared     = "service_dns_not_declared"
	reasonServiceNotCaddyManaged    = "service_not_caddy_managed"
	reasonNodeDisabled              = "node_disabled"
	reasonNodeOffline               = "node_offline"
	reasonNodeNotEligible           = "node_not_eligible"
	reasonNodeNotRusticManaged      = "node_not_rustic_managed"
	reasonMissingEligibleRusticNode = "missing_eligible_rustic_node"
	reasonMissingOnlineRusticNode   = "missing_online_rustic_node"
	reasonBackupNotSucceeded        = "backup_not_succeeded"
	reasonBackupArtifactMissing     = "backup_artifact_missing"
	reasonMissingRestoreTargetNode  = "missing_restore_target_node"
)

func (server *systemServer) GetCapabilities(ctx context.Context, _ *connect.Request[controllerv1.GetCapabilitiesRequest]) (*connect.Response[controllerv1.GetCapabilitiesResponse], error) {
	snapshotByNodeID, err := buildNodeSnapshotMap(ctx, server.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&controllerv1.GetCapabilitiesResponse{
		Global: buildGlobalCapabilities(server.cfg, server.availableNodeIDs, snapshotByNodeID),
	}), nil
}

func buildGlobalCapabilities(cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, snapshotByNodeID map[string]store.NodeSnapshot) *controllerv1.GlobalCapabilities {
	return &controllerv1.GlobalCapabilities{
		Backup:            backupIntegrationCapability(cfg, availableNodeIDs),
		Dns:               dnsIntegrationCapability(cfg),
		Secrets:           secretsCapability(cfg),
		RusticMaintenance: rusticMaintenanceGlobalCapability(cfg, availableNodeIDs, snapshotByNodeID),
	}
}

func buildDisabledServiceActionCapabilities(reason string) *controllerv1.ServiceActionCapabilities {
	return &controllerv1.ServiceActionCapabilities{
		Backup:    disabledCapability(reason),
		Restore:   disabledCapability(reason),
		Migrate:   disabledCapability(reason),
		DnsUpdate: disabledCapability(reason),
		CaddySync: disabledCapability(reason),
	}
}

func buildServiceActionCapabilities(cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, snapshotByNodeID map[string]store.NodeSnapshot, service repo.Service) *controllerv1.ServiceActionCapabilities {
	return &controllerv1.ServiceActionCapabilities{
		Backup:    serviceBackupCapability(cfg, availableNodeIDs, snapshotByNodeID, service),
		Restore:   serviceRestoreCapability(cfg, availableNodeIDs, service),
		Migrate:   serviceMigrateCapability(cfg, availableNodeIDs, service),
		DnsUpdate: serviceDNSUpdateCapability(cfg, service),
		CaddySync: serviceCaddySyncCapability(cfg, snapshotByNodeID, service),
	}
}

func buildNodeActionCapabilities(cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, snapshotByNodeID map[string]store.NodeSnapshot, nodeID string) *controllerv1.NodeActionCapabilities {
	return &controllerv1.NodeActionCapabilities{
		CaddySync:         nodeCaddySyncCapability(cfg, snapshotByNodeID, nodeID),
		CaddyReload:       nodeCaddyReloadCapability(cfg, availableNodeIDs, snapshotByNodeID, nodeID),
		RusticMaintenance: nodeRusticMaintenanceCapability(cfg, availableNodeIDs, snapshotByNodeID, nodeID),
	}
}

func buildBackupActionCapabilities(cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, snapshotByNodeID map[string]store.NodeSnapshot, backup store.BackupDetail) *controllerv1.BackupActionCapabilities {
	return &controllerv1.BackupActionCapabilities{
		Restore: backupRestoreCapability(cfg, availableNodeIDs, snapshotByNodeID, backup),
	}
}

func buildNodeSnapshotMap(ctx context.Context, db *store.DB) (map[string]store.NodeSnapshot, error) {
	snapshots, err := db.ListNodeSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	snapshotByNodeID := make(map[string]store.NodeSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		snapshotByNodeID[snapshot.NodeID] = snapshot
	}
	return snapshotByNodeID, nil
}

func enabledCapability() *controllerv1.Capability {
	return &controllerv1.Capability{Enabled: true}
}

func disabledCapability(reason string) *controllerv1.Capability {
	return &controllerv1.Capability{Enabled: false, ReasonCode: reason}
}

func backupIntegrationCapability(cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}) *controllerv1.Capability {
	if cfg == nil {
		return disabledCapability(reasonMissingBackupIntegration)
	}
	if _, err := repo.FindRusticInfraService(cfg.RepoDir, availableNodeIDs); err != nil {
		return disabledCapability(reasonMissingBackupIntegration)
	}
	return enabledCapability()
}

func dnsIntegrationCapability(cfg *config.ControllerConfig) *controllerv1.Capability {
	if cfg == nil || cfg.DNS == nil || cfg.DNS.Cloudflare == nil || strings.TrimSpace(cfg.DNS.Cloudflare.APITokenFile) == "" {
		return disabledCapability(reasonMissingDNSIntegration)
	}
	return enabledCapability()
}

func secretsCapability(cfg *config.ControllerConfig) *controllerv1.Capability {
	if cfg == nil || cfg.Secrets == nil {
		return disabledCapability(reasonMissingSecretsConfig)
	}
	if cfg.Secrets.Provider != "age" || strings.TrimSpace(cfg.Secrets.IdentityFile) == "" {
		return disabledCapability(reasonMissingSecretsConfig)
	}
	return enabledCapability()
}

func rusticMaintenanceGlobalCapability(cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, snapshotByNodeID map[string]store.NodeSnapshot) *controllerv1.Capability {
	if cfg == nil {
		return disabledCapability(reasonMissingBackupIntegration)
	}
	rusticService, err := repo.FindRusticInfraService(cfg.RepoDir, availableNodeIDs)
	if err != nil {
		return disabledCapability(reasonMissingBackupIntegration)
	}
	candidates := append([]string(nil), rusticService.TargetNodes...)
	if cfg != nil && cfg.Rustic != nil && len(cfg.Rustic.MainNodes) > 0 {
		allowed := make(map[string]struct{}, len(cfg.Rustic.MainNodes))
		for _, nodeID := range cfg.Rustic.MainNodes {
			allowed[nodeID] = struct{}{}
		}
		filtered := make([]string, 0, len(candidates))
		for _, nodeID := range candidates {
			if _, ok := allowed[nodeID]; ok {
				filtered = append(filtered, nodeID)
			}
		}
		candidates = filtered
	}
	if len(candidates) == 0 {
		return disabledCapability(reasonMissingEligibleRusticNode)
	}
	for _, nodeID := range candidates {
		if taskTargetNodeCapability(cfg, snapshotByNodeID, nodeID, task.TypeRusticInit).GetEnabled() {
			return enabledCapability()
		}
	}
	return disabledCapability(reasonMissingOnlineRusticNode)
}

func serviceBackupCapability(cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, snapshotByNodeID map[string]store.NodeSnapshot, service repo.Service) *controllerv1.Capability {
	if capability := backupIntegrationCapability(cfg, availableNodeIDs); !capability.GetEnabled() {
		return capability
	}
	if len(repo.EnabledBackupDataNames(service)) == 0 {
		return disabledCapability(reasonMissingBackupDefinition)
	}
	return taskTargetNodesCapability(cfg, snapshotByNodeID, service.TargetNodes, task.TypeBackup)
}

func serviceRestoreCapability(cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, service repo.Service) *controllerv1.Capability {
	if capability := backupIntegrationCapability(cfg, availableNodeIDs); !capability.GetEnabled() {
		return capability
	}
	if !serviceHasRestoreDefinition(service, "") {
		return disabledCapability(reasonMissingRestoreDefinition)
	}
	return enabledCapability()
}

func serviceMigrateCapability(cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, service repo.Service) *controllerv1.Capability {
	if capability := backupIntegrationCapability(cfg, availableNodeIDs); !capability.GetEnabled() {
		return capability
	}
	if len(enabledMigrateDataNames(service)) == 0 {
		return disabledCapability(reasonMissingMigrateDefinition)
	}
	return enabledCapability()
}

func serviceDNSUpdateCapability(cfg *config.ControllerConfig, service repo.Service) *controllerv1.Capability {
	if service.Meta.Network == nil || service.Meta.Network.DNS == nil {
		return disabledCapability(reasonServiceDNSNotDeclared)
	}
	if capability := dnsIntegrationCapability(cfg); !capability.GetEnabled() {
		return capability
	}
	return enabledCapability()
}

func serviceCaddySyncCapability(cfg *config.ControllerConfig, snapshotByNodeID map[string]store.NodeSnapshot, service repo.Service) *controllerv1.Capability {
	if !repo.CaddyManaged(service) {
		return disabledCapability(reasonServiceNotCaddyManaged)
	}
	return taskTargetNodesCapability(cfg, snapshotByNodeID, service.TargetNodes, task.TypeCaddySync)
}

func backupRestoreCapability(cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, snapshotByNodeID map[string]store.NodeSnapshot, backup store.BackupDetail) *controllerv1.Capability {
	if backup.Status != string(task.StatusSucceeded) {
		return disabledCapability(reasonBackupNotSucceeded)
	}
	if strings.TrimSpace(backup.ArtifactRef) == "" {
		return disabledCapability(reasonBackupArtifactMissing)
	}
	if capability := backupIntegrationCapability(cfg, availableNodeIDs); !capability.GetEnabled() {
		return capability
	}
	service, err := repo.FindService(cfg.RepoDir, availableNodeIDs, backup.ServiceName)
	if err != nil {
		return disabledCapability(reasonServiceNotDeclared)
	}
	if !serviceHasRestoreDefinition(service, backup.DataName) {
		return disabledCapability(reasonMissingRestoreDefinition)
	}
	if !anyNodeEligibleForTask(cfg, snapshotByNodeID, task.TypeRestore) {
		return disabledCapability(reasonMissingRestoreTargetNode)
	}
	return enabledCapability()
}

func nodeCaddySyncCapability(cfg *config.ControllerConfig, snapshotByNodeID map[string]store.NodeSnapshot, nodeID string) *controllerv1.Capability {
	return taskTargetNodeCapability(cfg, snapshotByNodeID, nodeID, task.TypeCaddySync)
}

func nodeCaddyReloadCapability(cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, snapshotByNodeID map[string]store.NodeSnapshot, nodeID string) *controllerv1.Capability {
	if capability := taskTargetNodeCapability(cfg, snapshotByNodeID, nodeID, task.TypeCaddyReload); !capability.GetEnabled() {
		return capability
	}
	if _, err := repo.FindCaddyInfraService(cfg.RepoDir, availableNodeIDs); err != nil {
		return disabledCapability(reasonMissingCaddyInfra)
	}
	return enabledCapability()
}

func nodeRusticMaintenanceCapability(cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, snapshotByNodeID map[string]store.NodeSnapshot, nodeID string) *controllerv1.Capability {
	if capability := taskTargetNodeCapability(cfg, snapshotByNodeID, nodeID, task.TypeRusticInit); !capability.GetEnabled() {
		return capability
	}
	rusticService, err := repo.FindRusticInfraService(cfg.RepoDir, availableNodeIDs)
	if err != nil {
		return disabledCapability(reasonMissingBackupIntegration)
	}
	if !slices.Contains(rusticService.TargetNodes, nodeID) {
		return disabledCapability(reasonNodeNotRusticManaged)
	}
	return enabledCapability()
}

func taskTargetNodesCapability(cfg *config.ControllerConfig, snapshotByNodeID map[string]store.NodeSnapshot, nodeIDs []string, taskType task.Type) *controllerv1.Capability {
	if len(nodeIDs) == 0 {
		return disabledCapability(reasonNodeNotEligible)
	}
	for _, nodeID := range nodeIDs {
		if capability := taskTargetNodeCapability(cfg, snapshotByNodeID, nodeID, taskType); !capability.GetEnabled() {
			return capability
		}
	}
	return enabledCapability()
}

func taskTargetNodeCapability(cfg *config.ControllerConfig, snapshotByNodeID map[string]store.NodeSnapshot, nodeID string, taskType task.Type) *controllerv1.Capability {
	node, ok := lookupConfiguredNode(cfg, nodeID)
	if !ok {
		return disabledCapability(reasonNodeNotEligible)
	}
	if node.Enabled != nil && !*node.Enabled {
		return disabledCapability(reasonNodeDisabled)
	}
	if !task.RequiresOnlineNode(taskType) {
		return enabledCapability()
	}
	snapshot, ok := snapshotByNodeID[nodeID]
	if !ok || !snapshot.IsOnline {
		return disabledCapability(reasonNodeOffline)
	}
	return enabledCapability()
}

func anyNodeEligibleForTask(cfg *config.ControllerConfig, snapshotByNodeID map[string]store.NodeSnapshot, taskType task.Type) bool {
	if cfg == nil {
		return false
	}
	for _, node := range cfg.Nodes {
		if taskTargetNodeCapability(cfg, snapshotByNodeID, node.ID, taskType).GetEnabled() {
			return true
		}
	}
	return false
}

func lookupConfiguredNode(cfg *config.ControllerConfig, nodeID string) (config.NodeConfig, bool) {
	if cfg == nil {
		return config.NodeConfig{}, false
	}
	for _, node := range cfg.Nodes {
		if node.ID == nodeID {
			return node, true
		}
	}
	return config.NodeConfig{}, false
}

func serviceHasRestoreDefinition(service repo.Service, dataName string) bool {
	if service.Meta.DataProtect == nil {
		return false
	}
	for _, item := range service.Meta.DataProtect.Data {
		if dataName != "" && item.Name != dataName {
			continue
		}
		if item.Restore != nil {
			return true
		}
	}
	return false
}
