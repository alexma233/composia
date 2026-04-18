import type { Dictionary } from "$lib/i18n/messages/en-us";
import type {
  BackupActionCapabilities,
  Capability,
  GlobalCapabilities,
  NodeActionCapabilities,
  ServiceActionCapabilities,
} from "$lib/server/controller";

export function capabilityReasonMessage(
  reasonCode: string | null | undefined,
  dictionary: Dictionary,
): string {
  switch (reasonCode) {
    case "missing_backup_integration":
      return dictionary.capabilities.reasons.missingBackupIntegration;
    case "missing_backup_definition":
      return dictionary.capabilities.reasons.missingBackupDefinition;
    case "missing_restore_definition":
      return dictionary.capabilities.reasons.missingRestoreDefinition;
    case "missing_migrate_definition":
      return dictionary.capabilities.reasons.missingMigrateDefinition;
    case "missing_dns_integration":
      return dictionary.capabilities.reasons.missingDnsIntegration;
    case "missing_secrets_config":
      return dictionary.capabilities.reasons.missingSecretsConfig;
    case "missing_caddy_infra":
      return dictionary.capabilities.reasons.missingCaddyInfra;
    case "missing_service_meta":
      return dictionary.capabilities.reasons.missingServiceMeta;
    case "service_not_declared":
      return dictionary.capabilities.reasons.serviceNotDeclared;
    case "service_dns_not_declared":
      return dictionary.capabilities.reasons.serviceDnsNotDeclared;
    case "service_not_caddy_managed":
      return dictionary.capabilities.reasons.serviceNotCaddyManaged;
    case "node_disabled":
      return dictionary.capabilities.reasons.nodeDisabled;
    case "node_offline":
      return dictionary.capabilities.reasons.nodeOffline;
    case "node_not_eligible":
      return dictionary.capabilities.reasons.nodeNotEligible;
    case "node_not_rustic_managed":
      return dictionary.capabilities.reasons.nodeNotRusticManaged;
    case "missing_eligible_rustic_node":
      return dictionary.capabilities.reasons.missingEligibleRusticNode;
    case "missing_online_rustic_node":
      return dictionary.capabilities.reasons.missingOnlineRusticNode;
    case "backup_not_succeeded":
      return dictionary.capabilities.reasons.backupNotSucceeded;
    case "backup_artifact_missing":
      return dictionary.capabilities.reasons.backupArtifactMissing;
    case "missing_restore_target_node":
      return dictionary.capabilities.reasons.missingRestoreTargetNode;
    default:
      return dictionary.capabilities.reasons.unavailable;
  }
}

export function actionErrorMessage(
  payload: { reasonCode?: string; error?: string } | null | undefined,
  dictionary: Dictionary,
  fallback: string,
): string {
  if (payload?.reasonCode) {
    return capabilityReasonMessage(payload.reasonCode, dictionary);
  }
  return payload?.error ?? fallback;
}

export function serviceActionCapability(
  actions: ServiceActionCapabilities | null | undefined,
  action: keyof ServiceActionCapabilities,
): Capability {
  return actions?.[action] ?? { enabled: false, reasonCode: "" };
}

export function nodeActionCapability(
  actions: NodeActionCapabilities | null | undefined,
  action: keyof NodeActionCapabilities,
): Capability {
  return actions?.[action] ?? { enabled: false, reasonCode: "" };
}

export function globalCapability(
  capabilities: GlobalCapabilities | null | undefined,
  action: keyof GlobalCapabilities,
): Capability {
  return capabilities?.[action] ?? { enabled: false, reasonCode: "" };
}

export function backupActionCapability(
  actions: BackupActionCapabilities | null | undefined,
  action: keyof BackupActionCapabilities,
): Capability {
  return actions?.[action] ?? { enabled: false, reasonCode: "" };
}
