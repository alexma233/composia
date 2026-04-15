import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig, inspectNodeContainer } from "$lib/server/controller";

export const GET: RequestHandler = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason, rawJson: null }, { status: 503 });
  }

  try {
    const rawJson = await inspectNodeContainer(
      params.id,
      decodeURIComponent(params.cid),
    );
    return json({ rawJson });
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to inspect container",
        rawJson: null,
      },
      { status: 500 },
    );
  }
};
