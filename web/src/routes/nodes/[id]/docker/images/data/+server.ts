import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig, listNodeImages } from "$lib/server/controller";

export const GET: RequestHandler = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason, images: [] }, { status: 503 });
  }

  try {
    const images = await listNodeImages(params.id);
    return json({ images });
  } catch (error) {
    return json(
      {
        error: error instanceof Error ? error.message : "Failed to load images",
        images: [],
      },
      { status: 500 },
    );
  }
};
