import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { loadServiceInstance } from "$lib/server/controller";
import { jsonControllerError } from "$lib/server/controller-route";

export const GET: RequestHandler = async ({ params }) => {
  try {
    const instance = await loadServiceInstance(
      params.name,
      params.nodeId,
      true,
    );
    if (!instance) {
      return json({ error: "Service instance not found." }, { status: 404 });
    }
    return json({ instance });
  } catch (error) {
    return jsonControllerError(error, "Failed to load service instance.");
  }
};
