import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig, listNodeContainers } from "$lib/server/controller";
import {
  jsonControllerError,
  parseDockerListQuery,
} from "$lib/server/controller-route";

export const GET: RequestHandler = async ({ params, url }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json(
      { error: config.reason, containers: [], totalCount: 0 },
      { status: 503 },
    );
  }

  try {
    const result = await listNodeContainers(
      params.id,
      parseDockerListQuery(url),
    );
    return json({ containers: result.items, totalCount: result.totalCount });
  } catch (error) {
    return jsonControllerError(error, "Failed to load containers", {
      containers: [],
      totalCount: 0,
    });
  }
};
