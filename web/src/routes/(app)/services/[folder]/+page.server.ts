import type { PageServerLoad } from "./$types";

import { controllerConfig, loadRepoHead } from "$lib/server/controller";
import { loadServiceWorkspaces } from "$lib/server/service-index";
import {
  defaultServiceFilePath,
  normalizeServiceRelativePath,
} from "$lib/service-workspace";
import { loadServiceWorkspaceFile } from "$lib/server/service-workspace";
import { setDecryptedSecretResponseHeaders } from "$lib/server/secret-response";
import { loadServiceWorkspaceSummary } from "$lib/server/service-workspace-route";
import { isEncryptedFilePath } from "$lib/service-workspace";

export const load: PageServerLoad = async ({ params, setHeaders, url }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      workspace: null,
      tasks: [],
      backups: [],
      serviceDetail: null,
      imageUpdateChecks: [],
      services: [],
      nodeContainers: [],
      repoHead: null,
      fileTree: [],
      initialFile: null,
    };
  }

  try {
    const [summary, repoHead, services] = await Promise.all([
      loadServiceWorkspaceSummary(params.folder),
      loadRepoHead(),
      loadServiceWorkspaces(),
    ]);
    const {
      workspace,
      tasks,
      backups,
      serviceDetail,
      imageUpdateChecks,
      fileTree,
    } = summary;
    const nodeContainers = serviceDetail?.instances ?? [];
    const requestedFile = url.searchParams.get("file") ?? "";
    const activeFilePath = requestedFile
      ? normalizeServiceRelativePath(requestedFile)
      : defaultServiceFilePath(fileTree);
    const initialFile = activeFilePath
      ? await loadServiceWorkspaceFile(workspace.folder, activeFilePath)
      : null;
    setDecryptedSecretResponseHeaders(
      setHeaders,
      Boolean(activeFilePath && isEncryptedFilePath(activeFilePath)),
    );

    return {
      ready: true,
      error: null,
      workspace,
      tasks,
      backups,
      serviceDetail,
      imageUpdateChecks,
      services,
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
      imageUpdateChecks: [],
      services: [],
      nodeContainers: [],
      repoHead: null,
      fileTree: [],
      initialFile: null,
    };
  }
};
