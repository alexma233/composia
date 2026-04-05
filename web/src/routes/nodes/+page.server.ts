import type { PageServerLoad } from "./$types";

import { controllerConfig, loadNodes } from "$lib/server/controller";

export const load: PageServerLoad = async () => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, nodes: [] };
  }

  try {
    return {
      ready: true,
      error: null,
      nodes: await loadNodes(),
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : "Failed to load nodes.",
      nodes: [],
    };
  }
};
