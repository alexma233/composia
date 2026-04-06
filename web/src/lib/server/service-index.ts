import { parse } from "yaml";

import type { ServiceDetail, ServiceSummary } from "$lib/server/controller";
import {
  loadRepoEntries,
  loadRepoFile,
  loadServiceDetail,
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
  node: string;
  enabled: boolean;
};

type ParsedMeta = {
  name?: string;
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
  const details = await Promise.all(
    summaries.map((summary) => loadDetail(summary)),
  );
  const detailsByDirectory = new Map<
    string,
    { summary: ServiceSummary; detail: ServiceDetail }
  >();

  for (const item of details) {
    if (!item) {
      continue;
    }
    detailsByDirectory.set(item.detail.directory, item);
  }

  const workspaces = await Promise.all(
    directories.map(async (entry) => {
      const declared = detailsByDirectory.get(entry.path);
      if (declared) {
        return {
          folder: entry.path,
          displayName: declared.detail.name,
          serviceName: declared.detail.name,
          hasMeta: true,
          isDeclared: true,
          runtimeStatus: declared.detail.runtimeStatus,
          updatedAt: declared.detail.updatedAt,
          node: declared.detail.node,
          enabled: declared.detail.enabled,
        } satisfies ServiceWorkspaceSummary;
      }

      const meta = await loadMeta(entry.path);
      return {
        folder: entry.path,
        displayName: meta.parsed?.name?.trim() || entry.name,
        serviceName: meta.parsed?.name?.trim() || "",
        hasMeta: meta.exists,
        isDeclared: false,
        runtimeStatus: meta.exists ? "needs_validation" : "uninitialized",
        updatedAt: "",
        node: "",
        enabled: Boolean(meta.parsed?.name),
      } satisfies ServiceWorkspaceSummary;
    }),
  );

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

async function loadDetail(summary: ServiceSummary) {
  try {
    return {
      summary,
      detail: await loadServiceDetail(summary.name),
    };
  } catch (error) {
    console.error(
      `Failed to load service detail for ${summary.name}:`,
      error instanceof Error ? error.message : error,
    );
    return null;
  }
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
