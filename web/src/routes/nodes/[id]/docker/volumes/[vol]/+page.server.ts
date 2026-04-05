import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params }) => {
  return {
    nodeId: params.id,
    volumeName: decodeURIComponent(params.vol),
    rawJson: null,
    error: "Docker inspect not yet implemented",
  };
};
