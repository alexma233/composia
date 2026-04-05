import type { PageServerLoad } from './$types';

import { controllerConfig, inspectNodeNetwork } from "$lib/server/controller";

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      nodeId: params.id,
      networkId: decodeURIComponent(params.nid),
      rawJson: null,
    };
  }

  try {
    const rawJson = await inspectNodeNetwork(params.id, decodeURIComponent(params.nid));
    return {
      ready: true,
      error: null,
      nodeId: params.id,
      networkId: decodeURIComponent(params.nid),
      rawJson,
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : "Failed to inspect network",
      nodeId: params.id,
      networkId: decodeURIComponent(params.nid),
      rawJson: null,
    };
  }
};
