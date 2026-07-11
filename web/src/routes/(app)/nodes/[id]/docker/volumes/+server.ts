import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig, listNodeVolumes } from "$lib/server/controller";
import {
  jsonControllerError,
  parseDockerListQuery,
} from "$lib/server/controller-route";

export const GET: RequestHandler = async ({ params, url }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json(
      { error: config.reason, volumes: [], totalCount: 0 },
      { status: 503 },
    );
  }

  try {
    const result = await listNodeVolumes(params.id, parseDockerListQuery(url));
    return json({ volumes: result.items, totalCount: result.totalCount });
  } catch (error) {
    return jsonControllerError(error, "Failed to load volumes", {
      volumes: [],
      totalCount: 0,
    });
  }
};
