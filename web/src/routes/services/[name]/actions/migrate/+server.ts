import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { migrateService } from "$lib/server/controller";
import {
  jsonCapabilityError,
  jsonControllerError,
} from "$lib/server/controller-route";
import { requireDeclaredWorkspace } from "$lib/server/service-workspace-route";

export const POST: RequestHandler = async ({ params, request }) => {
  try {
    const payload = (await request.json()) as {
      sourceNodeId?: string;
      targetNodeId?: string;
    };
    if (!payload.sourceNodeId || !payload.targetNodeId) {
      return json(
        { error: "Source and target node IDs are required." },
        { status: 400 },
      );
    }

    const workspace = await requireDeclaredWorkspace(params.name);
    if (!workspace.actions.migrate.enabled) {
      return jsonCapabilityError(
        workspace.actions.migrate.reasonCode,
        "Migrate is unavailable.",
      );
    }
    return json(
      await migrateService(
        workspace.serviceName,
        payload.sourceNodeId,
        payload.targetNodeId,
      ),
    );
  } catch (error) {
    return jsonControllerError(error, "Failed to migrate service.", {}, 400);
  }
};
