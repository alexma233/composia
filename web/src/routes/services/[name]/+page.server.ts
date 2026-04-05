import type { PageServerLoad } from "./$types";

import {
  controllerConfig,
  loadRepoHead,
  loadServiceBackups,
  loadServiceTasks,
} from "$lib/server/controller";
import {
  defaultServiceFilePath,
  normalizeServiceRelativePath,
} from "$lib/service-workspace";
import {
  loadServiceFileTree,
  loadServiceWorkspaceFile,
} from "$lib/server/service-workspace";
import { loadServiceWorkspaces } from "$lib/server/service-index";

export const load: PageServerLoad = async ({ params, url }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      workspace: null,
      tasks: [],
      backups: [],
      repoHead: null,
      fileTree: [],
      initialFile: null,
    };
  }

  try {
    const [workspaces, repoHead] = await Promise.all([
      loadServiceWorkspaces(),
      loadRepoHead(),
    ]);
    const workspace = workspaces.find((item) => item.folder === params.name);
    if (!workspace) {
      return {
        ready: true,
        error: `Service folder ${params.name} was not found.`,
        workspace: null,
        tasks: [],
        backups: [],
        repoHead,
        fileTree: [],
        initialFile: null,
      };
    }

    const [tasks, backups, fileTree] = await Promise.all([
      workspace.isDeclared && workspace.serviceName
        ? loadServiceTasks(workspace.serviceName)
        : Promise.resolve([]),
      workspace.isDeclared && workspace.serviceName
        ? loadServiceBackups(workspace.serviceName)
        : Promise.resolve([]),
      loadServiceFileTree(workspace.folder),
    ]);
    const requestedFile = url.searchParams.get("file") ?? "";
    const activeFilePath = requestedFile
      ? normalizeServiceRelativePath(requestedFile)
      : defaultServiceFilePath(fileTree);
    const initialFile = activeFilePath
      ? await loadServiceWorkspaceFile(workspace.folder, activeFilePath)
      : null;

    return {
      ready: true,
      error: null,
      workspace,
      tasks,
      backups,
      repoHead,
      fileTree,
      initialFile,
    };
  } catch (error) {
    return {
      ready: true,
      error:
        error instanceof Error
          ? error.message
          : "Failed to load service detail.",
      workspace: null,
      tasks: [],
      backups: [],
      repoHead: null,
      fileTree: [],
      initialFile: null,
    };
  }
};
