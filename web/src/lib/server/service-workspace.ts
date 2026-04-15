import type { RepoFileEntry, RepoWriteResult } from "$lib/server/controller";
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
  return loadDirectoryTree(serviceDir, "");
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

async function loadDirectoryTree(
  serviceDir: string,
  relativeDir: string,
): Promise<ServiceFileNode[]> {
  const entries = await loadRepoEntries(
    repoPathForServicePath(serviceDir, relativeDir),
  );
  const visibleEntries = entries.filter((entry) => entry.name !== ".gitkeep");
  return Promise.all(visibleEntries.map((entry) => toNode(serviceDir, entry)));
}

async function toNode(
  serviceDir: string,
  entry: RepoFileEntry,
): Promise<ServiceFileNode> {
  const relativePath = serviceRelativePath(serviceDir, entry.path);
  return {
    name: entry.name,
    path: relativePath,
    isDir: entry.isDir,
    children: entry.isDir
      ? await loadDirectoryTree(serviceDir, relativePath)
      : [],
  };
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
