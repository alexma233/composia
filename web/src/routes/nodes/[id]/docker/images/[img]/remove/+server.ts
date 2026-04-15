import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig, removeNodeImage } from "$lib/server/controller";

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
    return json(
      {
        error:
          error instanceof Error ? error.message : "Failed to remove image",
      },
      { status: 500 },
    );
  }
};
