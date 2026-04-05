import type { PageServerLoad } from './$types';

import { controllerConfig, listNodeVolumes } from "$lib/server/controller";

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, nodeId: params.id, volumes: [] };
  }

  try {
    const volumes = await listNodeVolumes(params.id);
    return {
      ready: true,
      error: null,
      nodeId: params.id,
      volumes,
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : "Failed to load volumes",
      nodeId: params.id,
      volumes: [],
    };
  }
};
