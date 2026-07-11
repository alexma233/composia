import type { PageServerLoad } from "./$types";

import {
  controllerConfig,
  loadBackupDetail,
  loadNodes,
} from "$lib/server/controller";

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, backup: null, nodes: [] };
  }

  try {
    const [backup, nodes] = await Promise.all([
      loadBackupDetail(params.id),
      loadNodes(),
    ]);
    return {
      ready: true,
      error: null,
      backup,
      nodes,
    };
  } catch (error) {
    return {
      ready: true,
      error:
        error instanceof Error
          ? error.message
          : "Failed to load backup detail.",
      backup: null,
      nodes: [],
    };
  }
};
