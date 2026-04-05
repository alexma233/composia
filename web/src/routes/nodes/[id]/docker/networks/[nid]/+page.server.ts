import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params }) => {
  return {
    nodeId: params.id,
    networkId: decodeURIComponent(params.nid),
    rawJson: null,
    error: "Docker inspect not yet implemented",
  };
};
