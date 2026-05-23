import type { PageServerLoad } from "./$types";

import {
  controllerConfig,
  listRepoCommits,
  loadRepoHead,
  loadSystemCapabilities,
  loadSystemStatus,
} from "$lib/server/controller";

export const load: PageServerLoad = async () => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      system: null,
      repoHead: null,
      capabilities: null,
      initialCommits: { commits: [], nextCursor: "" },
    };
  }

  try {
    const [system, repoHead, capabilities, initialCommits] = await Promise.all([
      loadSystemStatus(),
      loadRepoHead(),
      loadSystemCapabilities(),
      listRepoCommits(10),
    ]);
    return {
      ready: true,
      error: null,
      system,
      repoHead,
      capabilities,
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
      initialCommits: { commits: [], nextCursor: "" },
    };
  }
};
