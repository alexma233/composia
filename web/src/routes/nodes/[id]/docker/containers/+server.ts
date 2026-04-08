import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig, listNodeContainers } from "$lib/server/controller";

export const GET: RequestHandler = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason, containers: [] }, { status: 503 });
  }

  try {
    const containers = await listNodeContainers(params.id);
    return json({ containers });
  } catch (error) {
    return json(
      {
        error: error instanceof Error ? error.message : "Failed to load containers",
        containers: [],
      },
      { status: 500 },
    );
  }
};
