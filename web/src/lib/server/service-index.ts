import { parse } from 'yaml';

import type { ServiceDetail, ServiceSummary } from '$lib/server/controller';
import {
  loadRepoEntries,
  loadRepoFile,
  loadServiceDetail,
  loadServices
} from '$lib/server/controller';

export type ServiceWorkspaceSummary = {
  folder: string;
  displayName: string;
  serviceName: string;
  hasMeta: boolean;
  runtimeStatus: string;
  updatedAt: string;
  node: string;
  enabled: boolean;
};

type ParsedMeta = {
  name?: string;
};

export async function loadServiceWorkspaces(): Promise<ServiceWorkspaceSummary[]> {
  const [rootEntries, summaries] = await Promise.all([loadRepoEntries(''), loadServices(200)]);
  const directories = rootEntries.filter((entry) => entry.isDir);
  const details = await Promise.all(summaries.map((summary) => loadDetail(summary)));
  const detailsByDirectory = new Map<string, { summary: ServiceSummary; detail: ServiceDetail }>();

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
          runtimeStatus: declared.detail.runtimeStatus,
          updatedAt: declared.detail.updatedAt,
          node: declared.detail.node,
          enabled: declared.detail.enabled
        } satisfies ServiceWorkspaceSummary;
      }

      const meta = await loadMeta(entry.path);
      return {
        folder: entry.path,
        displayName: meta?.name?.trim() || entry.name,
        serviceName: meta?.name?.trim() || '',
        hasMeta: Boolean(meta?.name?.trim()),
        runtimeStatus: meta?.name ? 'unknown' : 'uninitialized',
        updatedAt: '',
        node: '',
        enabled: Boolean(meta?.name)
      } satisfies ServiceWorkspaceSummary;
    })
  );

  workspaces.sort((left, right) => left.folder.localeCompare(right.folder));
  return workspaces;
}

async function loadDetail(summary: ServiceSummary) {
  try {
    return {
      summary,
      detail: await loadServiceDetail(summary.name)
    };
  } catch {
    return null;
  }
}

async function loadMeta(folder: string): Promise<ParsedMeta | null> {
  try {
    const file = await loadRepoFile(`${folder}/composia-meta.yaml`);
    const meta = parse(file.content) as ParsedMeta | null;
    return meta && typeof meta === 'object' ? meta : null;
  } catch {
    return null;
  }
}
