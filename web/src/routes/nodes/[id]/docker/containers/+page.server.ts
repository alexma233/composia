import type { PageServerLoad } from './$types';

import { controllerConfig, listNodeContainers } from "$lib/server/controller";

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, nodeId: params.id, containers: [] };
  }

  try {
    const containers = await listNodeContainers(params.id);
    return {
      ready: true,
      error: null,
      nodeId: params.id,
      containers,
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : "Failed to load containers",
      nodeId: params.id,
      containers: [],
    };
  }
};
