import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig, listNodeImages } from "$lib/server/controller";
import {
  jsonControllerError,
  parseDockerListQuery,
} from "$lib/server/controller-route";

export const GET: RequestHandler = async ({ params, url }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json(
      { error: config.reason, images: [], totalCount: 0 },
      { status: 503 },
    );
  }

  try {
    const result = await listNodeImages(params.id, parseDockerListQuery(url));
    return json({ images: result.items, totalCount: result.totalCount });
  } catch (error) {
    return jsonControllerError(error, "Failed to load images", {
      images: [],
      totalCount: 0,
    });
  }
};
