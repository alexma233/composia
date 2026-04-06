import type { PageServerLoad } from "./$types";

import { controllerConfig, inspectNodeVolume } from "$lib/server/controller";

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      nodeId: params.id,
      volumeName: decodeURIComponent(params.vol),
      rawJson: null,
    };
  }

  try {
    const rawJson = await inspectNodeVolume(
      params.id,
      decodeURIComponent(params.vol),
    );
    return {
      ready: true,
      error: null,
      nodeId: params.id,
      volumeName: decodeURIComponent(params.vol),
      rawJson,
    };
  } catch (error) {
    return {
      ready: true,
      error:
        error instanceof Error ? error.message : "Failed to inspect volume",
      nodeId: params.id,
      volumeName: decodeURIComponent(params.vol),
      rawJson: null,
    };
  }
};
