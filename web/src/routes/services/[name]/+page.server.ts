import type { PageServerLoad } from "./$types";

import {
  controllerConfig,
  loadRepoHead,
} from "$lib/server/controller";
import {
  defaultServiceFilePath,
  normalizeServiceRelativePath,
} from "$lib/service-workspace";
import { loadServiceWorkspaceFile } from "$lib/server/service-workspace";
import { loadServiceWorkspaceSummary } from "$lib/server/service-workspace-route";

export const load: PageServerLoad = async ({ params, url }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      workspace: null,
      tasks: [],
      backups: [],
      serviceDetail: null,
      nodeContainers: [],
      repoHead: null,
      fileTree: [],
      initialFile: null,
    };
  }

  try {
    const [summary, repoHead] = await Promise.all([
      loadServiceWorkspaceSummary(params.name),
      loadRepoHead(),
    ]);
    const { workspace, tasks, backups, serviceDetail, fileTree } = summary;
    const nodeContainers = serviceDetail?.instances ?? [];
    const requestedFile = url.searchParams.get("file") ?? "";
    const activeFilePath = requestedFile
      ? normalizeServiceRelativePath(requestedFile)
      : defaultServiceFilePath(fileTree);
    const initialFile = activeFilePath
      ? await loadServiceWorkspaceFile(workspace.serviceName ?? null, workspace.folder, activeFilePath)
      : null;

    return {
      ready: true,
      error: null,
      workspace,
      tasks,
      backups,
      serviceDetail,
      nodeContainers,
      repoHead,
      fileTree,
      initialFile,
    };
  } catch (error) {
    const message =
      error instanceof Response
        ? await error.text()
        : error instanceof Error
          ? error.message
          : "Failed to load service detail.";
    return {
      ready: true,
      error: message,
      workspace: null,
      tasks: [],
      backups: [],
      serviceDetail: null,
      nodeContainers: [],
      repoHead: null,
      fileTree: [],
      initialFile: null,
    };
  }
};
