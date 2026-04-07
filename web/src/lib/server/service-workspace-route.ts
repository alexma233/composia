import { json } from "@sveltejs/kit";

import {
  loadBackups,
  loadServiceDetail,
  loadTasks,
  type BackupSummary,
  type ServiceDetail,
  type TaskSummary,
} from "$lib/server/controller";
import {
  loadServiceWorkspace,
  type ServiceWorkspaceSummary,
} from "$lib/server/service-index";
import { loadServiceFileTree } from "$lib/server/service-workspace";
import type { ServiceFileNode } from "$lib/service-workspace";

export type ServiceWorkspaceSummaryData = {
  workspace: ServiceWorkspaceSummary;
  tasks: TaskSummary[];
  backups: BackupSummary[];
  serviceDetail: ServiceDetail | null;
  fileTree: ServiceFileNode[];
};

export async function requireWorkspace(folder: string) {
  const workspace = await loadServiceWorkspace(folder);
  if (!workspace) {
    throw new Response(JSON.stringify({ error: "Service folder not found." }), {
      status: 404,
      headers: { "content-type": "application/json" },
    });
  }
  return workspace;
}

export async function requireDeclaredWorkspace(folder: string) {
  const workspace = await requireWorkspace(folder);
  if (!workspace.isDeclared || !workspace.serviceName) {
    throw new Error(
      "Add a valid composia-meta.yaml for this folder before running service actions.",
    );
  }
  return workspace;
}

export async function loadServiceWorkspaceSummary(
  folder: string,
): Promise<ServiceWorkspaceSummaryData> {
  const workspace = await requireWorkspace(folder);
  const [tasksResult, backupsResult, serviceDetail, fileTree] = await Promise.all([
    workspace.isDeclared && workspace.serviceName
      ? loadTasks(1, 20, { serviceName: workspace.serviceName })
      : Promise.resolve({ items: [], totalCount: 0 }),
    workspace.isDeclared && workspace.serviceName
      ? loadBackups(1, 20, { serviceName: workspace.serviceName })
      : Promise.resolve({ items: [], totalCount: 0 }),
    workspace.isDeclared && workspace.serviceName
      ? loadServiceDetail(workspace.serviceName)
      : Promise.resolve(null),
    loadServiceFileTree(folder),
  ]);

  return {
    workspace,
    tasks: tasksResult.items,
    backups: backupsResult.items,
    serviceDetail,
    fileTree,
  };
}

export function jsonError(error: unknown, fallback: string, status = 400) {
  return json(
    {
      error: error instanceof Error ? error.message : fallback,
    },
    { status },
  );
}
