import { parse } from "yaml";

import type { ServiceSummary } from "$lib/server/controller";
import {
  loadRepoEntries,
  loadRepoFile,
  loadServices,
} from "$lib/server/controller";

export type ServiceWorkspaceSummary = {
  folder: string;
  displayName: string;
  serviceName: string;
  hasMeta: boolean;
  isDeclared: boolean;
  runtimeStatus: string;
  updatedAt: string;
  nodes: string[];
  node: string;
  enabled: boolean;
};

type ParsedMeta = {
  name?: string;
  enabled?: boolean;
  nodes?: string[];
};

type MetaInfo = {
  exists: boolean;
  parsed: ParsedMeta | null;
};

export async function loadServiceWorkspaces(): Promise<
  ServiceWorkspaceSummary[]
> {
  const [rootEntries, summariesResult] = await Promise.all([
    loadRepoEntries(""),
    loadServices(1, 200),
  ]);
  const summaries = summariesResult.items;
  const directories = rootEntries.filter((entry) => entry.isDir);
  const metas = await Promise.all(
    directories.map(async (entry) => ({
      entry,
      meta: await loadMeta(entry.path),
    })),
  );
  const summariesByServiceName = new Map(
    summaries.map((summary) => [summary.name, summary] as const),
  );

  const workspaces = metas.map(({ entry, meta }) => {
    const serviceName = meta.parsed?.name?.trim() || "";
    const declared = serviceName
      ? summariesByServiceName.get(serviceName)
      : undefined;
    if (declared) {
      return serviceSummaryWorkspace(entry.path, declared, serviceName, meta);
    }

    return {
      folder: entry.path,
      displayName: serviceName || entry.name,
      serviceName,
      hasMeta: meta.exists,
      isDeclared: false,
      runtimeStatus: meta.exists ? "needs_validation" : "uninitialized",
      updatedAt: "",
      nodes: normalizeNodes(meta.parsed?.nodes),
      node: normalizeNodes(meta.parsed?.nodes).join(", "),
      enabled: meta.parsed?.enabled ?? Boolean(serviceName),
    } satisfies ServiceWorkspaceSummary;
  });

  workspaces.sort((left, right) => left.folder.localeCompare(right.folder));
  return workspaces;
}

export async function loadServiceWorkspace(
  folder: string,
): Promise<ServiceWorkspaceSummary | null> {
  return (
    (await loadServiceWorkspaces()).find(
      (workspace) => workspace.folder === folder,
    ) ?? null
  );
}

function serviceSummaryWorkspace(
  folder: string,
  summary: ServiceSummary,
  serviceName: string,
  meta: MetaInfo,
): ServiceWorkspaceSummary {
  const nodes = normalizeNodes(meta.parsed?.nodes);
  return {
    folder,
    displayName: serviceName,
    serviceName,
    hasMeta: true,
    isDeclared: true,
    runtimeStatus: summary.runtimeStatus,
    updatedAt: summary.updatedAt,
    nodes,
    node: nodes.join(", "),
    enabled: meta.parsed?.enabled ?? true,
  };
}

function normalizeNodes(nodes: string[] | undefined): string[] {
  return (nodes ?? []).map((node) => node.trim()).filter(Boolean);
}

async function loadMeta(folder: string): Promise<MetaInfo> {
  try {
    const file = await loadRepoFile(`${folder}/composia-meta.yaml`);
    const meta = parse(file.content) as ParsedMeta | null;
    return {
      exists: true,
      parsed: meta && typeof meta === "object" ? meta : null,
    };
  } catch {
    return {
      exists: false,
      parsed: null,
    };
  }
}
