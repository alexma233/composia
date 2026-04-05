import type { PageServerLoad } from './$types';

import { controllerConfig, listNodeImages } from "$lib/server/controller";

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, nodeId: params.id, images: [] };
  }

  try {
    const images = await listNodeImages(params.id);
    return {
      ready: true,
      error: null,
      nodeId: params.id,
      images,
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : "Failed to load images",
      nodeId: params.id,
      images: [],
    };
  }
};
