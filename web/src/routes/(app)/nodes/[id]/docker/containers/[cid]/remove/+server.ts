import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig, removeNodeContainer } from "$lib/server/controller";
import { jsonControllerError } from "$lib/server/controller-route";

export const POST: RequestHandler = async ({ params, request }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason }, { status: 503 });
  }

  try {
    const payload = await request.json().catch(() => ({}));
    return json(
      await removeNodeContainer(params.id, decodeURIComponent(params.cid), {
        force: payload.force === true,
        removeVolumes: payload.removeVolumes === true,
      }),
    );
  } catch (error) {
    return jsonControllerError(error, "Failed to remove container");
  }
};
