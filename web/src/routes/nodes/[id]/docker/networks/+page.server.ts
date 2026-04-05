import type { PageServerLoad } from './$types';

import { controllerConfig } from "$lib/server/controller";

type NetworkSummary = {
  id: string;
  name: string;
  driver: string;
  scope: string;
  internal: boolean;
  attachable: boolean;
  created: string;
};

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, nodeId: params.id, networks: [] as NetworkSummary[] };
  }

  return {
    ready: true,
    error: null,
    nodeId: params.id,
    networks: [] as NetworkSummary[],
  };
};
