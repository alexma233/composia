import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig, removeNodeNetwork } from "$lib/server/controller";
import { jsonControllerError } from "$lib/server/controller-route";

export const POST: RequestHandler = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason }, { status: 503 });
  }

  try {
    return json(
      await removeNodeNetwork(params.id, decodeURIComponent(params.nid)),
    );
  } catch (error) {
    return jsonControllerError(error, "Failed to remove network");
  }
};
