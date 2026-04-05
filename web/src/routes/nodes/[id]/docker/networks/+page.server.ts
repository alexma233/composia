import type { PageServerLoad } from './$types';

import { controllerConfig, listNodeNetworks } from "$lib/server/controller";

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, nodeId: params.id, networks: [] };
  }

  try {
    const networks = await listNodeNetworks(params.id);
    return {
      ready: true,
      error: null,
      nodeId: params.id,
      networks,
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : "Failed to load networks",
      nodeId: params.id,
      networks: [],
    };
  }
};
