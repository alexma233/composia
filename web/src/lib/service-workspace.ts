export type ServiceFileNode = {
  name: string;
  path: string;
  isDir: boolean;
  iconName?: string;
  lightIconName?: string;
  expandedIconName?: string;
  expandedLightIconName?: string;
  children: ServiceFileNode[];
};

export type WorkspaceFile = {
  path: string;
  content: string;
  size: number;
  readOnly?: boolean;
  unavailableReasonCode?: string;
};

export function isEncryptedFilePath(path: string) {
  return path.toLowerCase().endsWith(".enc");
}

export function encryptedFileSourcePath(path: string) {
  return isEncryptedFilePath(path) ? path.slice(0, -4) : path;
}

export function normalizeServiceRelativePath(input: string) {
  const trimmed = input.trim().replaceAll("\\", "/");
  if (!trimmed) {
    return "";
  }

  const parts = trimmed.split("/").filter(Boolean);
  if (!parts.length) {
    return "";
  }
  if (parts.some((part) => part === "." || part === "..")) {
    throw new Error("Path must stay inside the current service directory.");
  }
  return parts.join("/");
}

export function defaultServiceFilePath(nodes: ServiceFileNode[]) {
  const preferred = findNode(nodes, "composia-meta.yaml");
  if (preferred) {
    return preferred.path;
  }
  return firstFile(nodes)?.path ?? "";
}

export function findNode(
  nodes: ServiceFileNode[],
  targetPath: string,
): ServiceFileNode | null {
  for (const node of nodes) {
    if (node.path === targetPath) {
      return node;
    }
    if (node.children.length) {
      const nested = findNode(node.children, targetPath);
      if (nested) {
        return nested;
      }
    }
  }
  return null;
}

function firstFile(nodes: ServiceFileNode[]): ServiceFileNode | null {
  for (const node of nodes) {
    if (!node.isDir) {
      return node;
    }
    const nested = firstFile(node.children);
    if (nested) {
      return nested;
    }
  }
  return null;
}
