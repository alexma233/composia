import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig, loadTaskDetail } from "$lib/server/controller";

export const GET: RequestHandler = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason, task: null }, { status: 503 });
  }

  try {
    return json({ task: await loadTaskDetail(params.id) });
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error ? error.message : "Failed to load task detail.",
        task: null,
      },
      { status: 500 },
    );
  }
};
