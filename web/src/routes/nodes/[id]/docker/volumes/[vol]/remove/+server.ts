import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig, removeNodeVolume } from "$lib/server/controller";

export const POST: RequestHandler = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason }, { status: 503 });
  }

  try {
    return json(
      await removeNodeVolume(params.id, decodeURIComponent(params.vol)),
    );
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error ? error.message : "Failed to remove volume",
      },
      { status: 500 },
    );
  }
};
