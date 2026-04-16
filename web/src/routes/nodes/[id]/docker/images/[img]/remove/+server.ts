import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig, removeNodeImage } from "$lib/server/controller";
import { jsonControllerError } from "$lib/server/controller-route";

export const POST: RequestHandler = async ({ params, request }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason }, { status: 503 });
  }

  try {
    const payload = await request.json().catch(() => ({}));
    return json(
      await removeNodeImage(
        params.id,
        decodeURIComponent(params.img),
        payload.force === true,
      ),
    );
  } catch (error) {
    return jsonControllerError(error, "Failed to remove image");
  }
};
