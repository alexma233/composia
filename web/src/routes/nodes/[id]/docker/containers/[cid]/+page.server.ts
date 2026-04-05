import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params }) => {
  return {
    nodeId: params.id,
    containerId: decodeURIComponent(params.cid),
    rawJson: null,
    error: "Docker inspect not yet implemented",
  };
};
