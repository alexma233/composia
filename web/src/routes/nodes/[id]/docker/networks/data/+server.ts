import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

import { controllerConfig, listNodeNetworks } from '$lib/server/controller';

export const GET: RequestHandler = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason, networks: [] }, { status: 503 });
  }

  try {
    const networks = await listNodeNetworks(params.id);
    return json({ networks });
  } catch (error) {
    return json(
      {
        error: error instanceof Error ? error.message : 'Failed to load networks',
        networks: [],
      },
      { status: 500 },
    );
  }
};
