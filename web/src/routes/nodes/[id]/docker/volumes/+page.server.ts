import type { PageServerLoad } from './$types';

import { controllerConfig, loadNodeDetail } from "$lib/server/controller";

type VolumeSummary = {
  name: string;
  driver: string;
  scope: string;
  mountpoint: string;
  created: string;
};

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, nodeId: params.id, volumes: [] as VolumeSummary[] };
  }

  return {
    ready: true,
    error: null,
    nodeId: params.id,
    volumes: [] as VolumeSummary[],
  };
};
