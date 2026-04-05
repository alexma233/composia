import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params }) => {
  return {
    nodeId: params.id,
    imageId: decodeURIComponent(params.img),
    rawJson: null,
    error: "Docker inspect not yet implemented",
  };
};
