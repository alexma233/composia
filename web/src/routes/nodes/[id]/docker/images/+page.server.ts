import type { PageServerLoad } from './$types';

import { controllerConfig } from "$lib/server/controller";

type ImageSummary = {
  id: string;
  repoTags: string[];
  size: number;
  created: string;
};

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, nodeId: params.id, images: [] as ImageSummary[] };
  }

  return {
    ready: true,
    error: null,
    nodeId: params.id,
    images: [] as ImageSummary[],
  };
};
