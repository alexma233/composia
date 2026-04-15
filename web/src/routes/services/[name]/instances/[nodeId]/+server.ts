import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { loadServiceInstance } from "$lib/server/controller";

export const GET: RequestHandler = async ({ params }) => {
  try {
    const instance = await loadServiceInstance(params.name, params.nodeId);
    if (!instance) {
      return json({ error: "Service instance not found." }, { status: 404 });
    }
    return json({ instance });
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to load service instance.",
      },
      { status: 400 },
    );
  }
};
