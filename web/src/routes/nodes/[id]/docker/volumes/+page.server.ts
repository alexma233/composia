import type { PageServerLoad } from "./$types";

import { controllerConfig } from "$lib/server/controller";

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      nodeId: params.id,
      volumes: [],
      initialLoaded: false,
    };
  }

  return {
    ready: true,
    error: null,
    nodeId: params.id,
    volumes: [],
    initialLoaded: false,
  };
};
