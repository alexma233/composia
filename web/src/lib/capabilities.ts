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
  payload:
    | { reasonCode?: string; errorCode?: string; error?: string }
    | null
    | undefined,
  dictionary: Dictionary,
  fallback: string,
): string {
  if (payload?.reasonCode) {
    return capabilityReasonMessage(payload.reasonCode, dictionary);
  }
  if (payload?.errorCode) {
    return (
      resolveApiErrorCode(payload.errorCode, dictionary) ??
      payload?.error ??
      fallback
    );
  }
  return payload?.error ?? fallback;
}

export function resolveApiErrorCode(
  code: string,
  dictionary: Dictionary,
): string | null {
  const key = API_ERROR_CODE_MAP[code];
  if (!key) return null;
  return (dictionary.apiError as Record<string, string>)[key] ?? null;
}

const API_ERROR_CODE_MAP: Record<string, string> = {
  AUTHENTICATION_REQUIRED: "authenticationRequired",
  UNSUPPORTED_SERVICE_ACTION: "unsupportedServiceAction",
  INVALID_CONFIRMATION_DECISION: "invalidConfirmationDecision",
  SERVICE_NOT_DECLARED: "serviceNotDeclared",
  SERVICE_INSTANCE_NOT_FOUND: "serviceInstanceNotFound",
  BASE_REVISION_REQUIRED: "baseRevisionRequired",
  PATH_REVISION_REQUIRED: "pathRevisionRequired",
  SOURCE_DEST_REVISION_REQUIRED: "sourceDestRevisionRequired",
  SOURCE_TARGET_NODE_REQUIRED: "sourceTargetNodeRequired",
  NODE_ID_REQUIRED: "nodeIdRequired",
  MOVE_PATH_FAILED: "movePathFailed",
  DELETE_PATH_FAILED: "deletePathFailed",
  SAVE_FILE_FAILED: "saveFileFailed",
  CREATE_DIRECTORY_FAILED: "createDirectoryFailed",
};

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
