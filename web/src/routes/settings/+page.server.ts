import type { PageServerLoad } from "./$types";

import {
  controllerConfig,
  listRepoCommits,
  loadCurrentConfig,
  loadRepoHead,
  loadSystemCapabilities,
  loadSystemStatus,
} from "$lib/server/controller";

export const load: PageServerLoad = async ({ depends }) => {
  depends("app:settings");
  const config = controllerConfig();
  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      system: null,
      repoHead: null,
      capabilities: null,
      currentConfig: null,
      initialCommits: { commits: [], nextCursor: "" },
    };
  }

  try {
    const [system, repoHead, capabilities, currentConfig, initialCommits] =
      await Promise.all([
        loadSystemStatus(),
        loadRepoHead(),
        loadSystemCapabilities(),
        loadCurrentConfig(),
        listRepoCommits(10),
      ]);
    return {
      ready: true,
      error: null,
      system,
      repoHead,
      capabilities,
      currentConfig,
      initialCommits,
    };
  } catch (error) {
    return {
      ready: true,
      error:
        error instanceof Error ? error.message : "Failed to load settings.",
      system: null,
      repoHead: null,
      capabilities: null,
      currentConfig: null,
      initialCommits: { commits: [], nextCursor: "" },
    };
  }
};
