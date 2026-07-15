import type { PageServerLoad } from "./$types";

import { svelteKitRouteParam } from "$lib/server/docker-route";

import { controllerConfig, inspectNodeNetwork } from "$lib/server/controller";

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      nodeId: params.id,
      networkId: svelteKitRouteParam(params.nid),
      rawJson: null,
    };
  }

  try {
    const rawJson = await inspectNodeNetwork(
      params.id,
      svelteKitRouteParam(params.nid),
    );
    return {
      ready: true,
      error: null,
      nodeId: params.id,
      networkId: svelteKitRouteParam(params.nid),
      rawJson,
    };
  } catch (error) {
    return {
      ready: true,
      error:
        error instanceof Error ? error.message : "Failed to inspect network",
      nodeId: params.id,
      networkId: svelteKitRouteParam(params.nid),
      rawJson: null,
    };
  }
};
