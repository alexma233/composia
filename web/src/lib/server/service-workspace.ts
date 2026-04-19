import type { RepoFileEntry, RepoWriteResult } from "$lib/server/controller";
import { resolveMaterialIconNames } from "$lib/server/vscode-material-icons";
import type { ServiceFileNode, WorkspaceFile } from "$lib/service-workspace";
import {
  createRepoDirectory,
  deleteRepoPath,
  loadRepoEntries,
  loadRepoFile,
  loadSystemCapabilities,
  loadSecret,
  moveRepoPath,
  updateRepoFile,
  updateSecret,
} from "$lib/server/controller";
import {
  isEncryptedFilePath,
  normalizeServiceRelativePath,
} from "$lib/service-workspace";

const ENCRYPTED_FILE_REASON_MESSAGES: Record<string, string> = {
  missing_secrets_config: "Secrets configuration is incomplete.",
  service_not_declared: "This service is not declared yet.",
};

export async function loadServiceFileTree(
  serviceDir: string,
): Promise<ServiceFileNode[]> {
  const entries = await loadRepoEntries(
    repoPathForServicePath(serviceDir, ""),
    {
      recursive: true,
    },
  );
  return buildServiceFileTree(serviceDir, entries);
}

export async function loadServiceWorkspaceFile(
  serviceName: string | null,
  serviceDir: string,
  relativePath: string,
): Promise<WorkspaceFile> {
  const normalized = normalizeServiceRelativePath(relativePath);
  let content: string;
  if (isEncryptedFilePath(normalized)) {
    const unavailableReasonCode = await resolveEncryptedFileUnavailableReason(
      serviceName,
    );
    if (unavailableReasonCode) {
      return unavailableEncryptedWorkspaceFile(normalized, unavailableReasonCode);
    }

    const declaredServiceName = serviceName as string;
    const secret = await loadSecret(declaredServiceName, normalized);
    content = secret.content ?? "";
  } else {
    const file = await loadRepoFile(
      repoPathForServicePath(serviceDir, normalized),
    );
    content = file.content ?? "";
  }
  return {
    path: normalized,
    content,
    size: content.length,
  };
}

export async function saveServiceWorkspaceFile(
  serviceName: string | null,
  serviceDir: string,
  relativePath: string,
  content: string,
  baseRevision: string,
): Promise<{ file: WorkspaceFile; write: RepoWriteResult }> {
  const normalized = normalizeServiceRelativePath(relativePath);
  let write: RepoWriteResult;
  if (isEncryptedFilePath(normalized)) {
    const unavailableReasonCode = await resolveEncryptedFileUnavailableReason(
      serviceName,
    );
    if (unavailableReasonCode) {
      throw new Error(
        ENCRYPTED_FILE_REASON_MESSAGES[unavailableReasonCode] ??
          "Encrypted file is currently unavailable.",
      );
    }

    const declaredServiceName = serviceName as string;
    write = await updateSecret(
      declaredServiceName,
      normalized,
      content,
      baseRevision,
    );
  } else {
    write = await updateRepoFile(
      repoPathForServicePath(serviceDir, normalized),
      content,
      baseRevision,
    );
  }
  return {
    file: {
      path: normalized,
      content,
      size: content.length,
    },
    write,
  };
}

async function resolveEncryptedFileUnavailableReason(
  serviceName: string | null,
) {
  if (!serviceName) {
    return "service_not_declared";
  }

  const capabilities = await loadSystemCapabilities();
  if (capabilities.global.secrets.enabled) {
    return null;
  }

  return capabilities.global.secrets.reasonCode || "missing_secrets_config";
}

function unavailableEncryptedWorkspaceFile(
  path: string,
  unavailableReasonCode: string,
): WorkspaceFile {
  return {
    path,
    content: "",
    size: 0,
    readOnly: true,
    unavailableReasonCode,
  };
}

export async function createServiceWorkspaceDirectory(
  serviceDir: string,
  relativePath: string,
  baseRevision: string,
): Promise<RepoWriteResult> {
  const normalized = normalizeServiceRelativePath(relativePath);
  return createRepoDirectory(
    repoPathForServicePath(serviceDir, normalized),
    baseRevision,
  );
}

export async function moveServiceWorkspacePath(
  serviceDir: string,
  sourcePath: string,
  destinationPath: string,
  baseRevision: string,
): Promise<RepoWriteResult> {
  const normalizedSource = normalizeServiceRelativePath(sourcePath);
  const normalizedDestination = normalizeServiceRelativePath(destinationPath);
  return moveRepoPath(
    repoPathForServicePath(serviceDir, normalizedSource),
    repoPathForServicePath(serviceDir, normalizedDestination),
    baseRevision,
  );
}

export async function deleteServiceWorkspacePath(
  serviceDir: string,
  relativePath: string,
  baseRevision: string,
): Promise<RepoWriteResult> {
  const normalized = normalizeServiceRelativePath(relativePath);
  return deleteRepoPath(
    repoPathForServicePath(serviceDir, normalized),
    baseRevision,
  );
}

function buildServiceFileTree(
  serviceDir: string,
  entries: RepoFileEntry[],
): ServiceFileNode[] {
  const root: ServiceFileNode[] = [];
  const directories = new Map<string, ServiceFileNode>();

  for (const entry of entries) {
    if (entry.name === ".gitkeep") {
      continue;
    }

    const relativePath = serviceRelativePath(serviceDir, entry.path);
    if (!relativePath) {
      continue;
    }

    const icons = resolveMaterialIconNames(relativePath, entry.isDir);

    const node: ServiceFileNode = {
      name: entry.name,
      path: relativePath,
      isDir: entry.isDir,
      iconName: icons.iconName ?? undefined,
      lightIconName: icons.lightIconName ?? undefined,
      expandedIconName: icons.expandedIconName ?? undefined,
      expandedLightIconName: icons.expandedLightIconName ?? undefined,
      children: [],
    };

    if (entry.isDir) {
      directories.set(relativePath, node);
    }

    const parentPath = parentServiceRelativePath(relativePath);
    if (!parentPath) {
      root.push(node);
      continue;
    }

    const parent = directories.get(parentPath);
    if (!parent) {
      throw new Error(
        `Repo path ${entry.path} is missing parent ${parentPath}.`,
      );
    }
    parent.children.push(node);
  }

  sortServiceFileNodes(root);
  return root;
}

function sortServiceFileNodes(nodes: ServiceFileNode[]) {
  nodes.sort((left, right) => {
    if (left.isDir !== right.isDir) {
      return left.isDir ? -1 : 1;
    }
    return left.name.localeCompare(right.name);
  });
  for (const node of nodes) {
    if (node.children.length > 0) {
      sortServiceFileNodes(node.children);
    }
  }
}

export function repoPathForServicePath(
  serviceDir: string,
  relativePath: string,
) {
  const normalizedDir =
    serviceDir === "." ? "" : normalizeServiceRelativePath(serviceDir);
  const normalizedPath = normalizeServiceRelativePath(relativePath);
  if (!normalizedDir) {
    return normalizedPath;
  }
  if (!normalizedPath) {
    return normalizedDir;
  }
  return `${normalizedDir}/${normalizedPath}`;
}

function serviceRelativePath(serviceDir: string, repoPath: string) {
  const normalizedDir =
    serviceDir === "." ? "" : normalizeServiceRelativePath(serviceDir);
  const normalizedRepoPath = normalizeServiceRelativePath(repoPath);
  if (!normalizedDir) {
    return normalizedRepoPath;
  }
  if (normalizedRepoPath === normalizedDir) {
    return "";
  }
  if (!normalizedRepoPath.startsWith(`${normalizedDir}/`)) {
    throw new Error(`Repo path ${repoPath} is outside service ${serviceDir}.`);
  }
  return normalizedRepoPath.slice(normalizedDir.length + 1);
}

function parentServiceRelativePath(path: string) {
  const normalized = normalizeServiceRelativePath(path);
  if (!normalized || !normalized.includes("/")) {
    return "";
  }
  return normalized.slice(0, normalized.lastIndexOf("/"));
}
