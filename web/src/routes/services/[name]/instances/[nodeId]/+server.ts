import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { loadServiceInstance } from "$lib/server/controller";
import { jsonControllerError } from "$lib/server/controller-route";
import { requireWorkspace } from "$lib/server/service-workspace-route";

export const GET: RequestHandler = async ({ params }) => {
  try {
    const workspace = await requireWorkspace(params.name);
    if (!workspace.isDeclared || !workspace.serviceName) {
      return json({ error: "Service is not declared." }, { status: 404 });
    }

    const instance = await loadServiceInstance(
      workspace.serviceName,
      params.nodeId,
      true,
    );
    if (!instance) {
      return json({ error: "Service instance not found." }, { status: 404 });
    }
    return json({ instance });
  } catch (error) {
    if (error instanceof Response) {
      return error;
    }
    return jsonControllerError(error, "Failed to load service instance.");
  }
};
