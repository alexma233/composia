import type { PageServerLoad } from "./$types";

import {
  controllerConfig,
  listRepoCommits,
  loadCurrentConfig,
  loadRepoHead,
  loadSystemStatus,
} from "$lib/server/controller";
import { isDocumentRequest } from "$lib/server/request";

export const load: PageServerLoad = async ({ depends, parent, request }) => {
  depends("app:settings");
  const config = controllerConfig();
  if (!config.ready) {
    return {
      content: {
        ready: false,
        error: config.reason,
        system: null,
        repoHead: null,
        capabilities: null,
        currentConfig: null,
        initialCommits: { commits: [], nextCursor: "" },
      },
    };
  }

  const parentData = await parent();
  const content = Promise.all([
    loadSystemStatus(),
    loadRepoHead(),
    Promise.resolve(parentData.capabilities),
    loadCurrentConfig(),
    listRepoCommits(10),
  ])
    .then(
      ([system, repoHead, capabilities, currentConfig, initialCommits]) => ({
        ready: true,
        error: null,
        system,
        repoHead,
        capabilities,
        currentConfig,
        initialCommits,
      }),
    )
    .catch((error: unknown) => ({
      ready: true,
      error:
        error instanceof Error ? error.message : "Failed to load settings.",
      system: null,
      repoHead: null,
      capabilities: null,
      currentConfig: null,
      initialCommits: { commits: [], nextCursor: "" },
    }));

  return { content: isDocumentRequest(request) ? content : await content };
};
