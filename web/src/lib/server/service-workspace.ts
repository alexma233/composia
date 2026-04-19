import type { RepoFileEntry, RepoWriteResult } from "$lib/server/controller";
import { resolveMaterialIconNames } from "$lib/server/vscode-material-icons";
import type { ServiceFileNode, WorkspaceFile } from "$lib/service-workspace";
import {
  createRepoDirectory,
  deleteRepoPath,
  loadRepoEntries,
  loadRepoFile,
  loadSecret,
  moveRepoPath,
  updateRepoFile,
  updateSecret,
} from "$lib/server/controller";
import { normalizeServiceRelativePath } from "$lib/service-workspace";

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
  if (normalized.endsWith(".enc")) {
    if (!serviceName) {
      throw new Error("Cannot load encrypted file for undeclared service");
    }
    const secret = await loadSecret(serviceName, normalized);
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
  if (normalized.endsWith(".enc")) {
    if (!serviceName) {
      throw new Error("Cannot save encrypted file for undeclared service");
    }
    write = await updateSecret(serviceName, normalized, content, baseRevision);
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
