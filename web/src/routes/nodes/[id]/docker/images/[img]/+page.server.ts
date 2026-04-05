import type { PageServerLoad } from './$types';

import { controllerConfig, inspectNodeImage } from "$lib/server/controller";

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      nodeId: params.id,
      imageId: decodeURIComponent(params.img),
      rawJson: null,
    };
  }

  try {
    const rawJson = await inspectNodeImage(params.id, decodeURIComponent(params.img));
    return {
      ready: true,
      error: null,
      nodeId: params.id,
      imageId: decodeURIComponent(params.img),
      rawJson,
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : "Failed to inspect image",
      nodeId: params.id,
      imageId: decodeURIComponent(params.img),
      rawJson: null,
    };
  }
};
