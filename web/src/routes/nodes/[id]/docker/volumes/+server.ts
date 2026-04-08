import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig, listNodeVolumes } from "$lib/server/controller";

export const GET: RequestHandler = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason, volumes: [] }, { status: 503 });
  }

  try {
    const volumes = await listNodeVolumes(params.id);
    return json({ volumes });
  } catch (error) {
    return json(
      {
        error: error instanceof Error ? error.message : "Failed to load volumes",
        volumes: [],
      },
      { status: 500 },
    );
  }
};
