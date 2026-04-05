import type { PageServerLoad } from './$types';

import { controllerConfig } from "$lib/server/controller";

type ContainerSummary = {
  id: string;
  name: string;
  image: string;
  state: string;
  status: string;
  created: string;
};

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, nodeId: params.id, containers: [] as ContainerSummary[] };
  }

  return {
    ready: true,
    error: null,
    nodeId: params.id,
    containers: [] as ContainerSummary[],
  };
};
