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
};

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

export function upsertFileNode(
  nodes: ServiceFileNode[],
  filePath: string,
): ServiceFileNode[] {
  const normalized = normalizeServiceRelativePath(filePath);
  if (!normalized) {
    return nodes;
  }

  const root = JSON.parse(JSON.stringify(nodes));
  const segments = normalized.split("/");
  let cursor = root;
  let currentPath = "";

  segments.forEach((segment, index) => {
    currentPath = currentPath ? `${currentPath}/${segment}` : segment;
    const isDir = index < segments.length - 1;
    let next = cursor.find((entry: ServiceFileNode) => entry.name === segment);
    if (!next) {
      next = { name: segment, path: currentPath, isDir, children: [] };
      cursor.push(next);
      cursor.sort((left: ServiceFileNode, right: ServiceFileNode) => {
        if (left.isDir !== right.isDir) {
          return left.isDir ? -1 : 1;
        }
        return left.name.localeCompare(right.name);
      });
    }
    cursor = next.children;
  });

  return root;
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
