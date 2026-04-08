import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { migrateService } from "$lib/server/controller";
import { requireDeclaredWorkspace } from "$lib/server/service-workspace-route";

export const POST: RequestHandler = async ({ params, request }) => {
  try {
    const payload = (await request.json()) as {
      sourceNodeId?: string;
      targetNodeId?: string;
    };
    if (!payload.sourceNodeId || !payload.targetNodeId) {
      return json({ error: "Source and target node IDs are required." }, { status: 400 });
    }

    const workspace = await requireDeclaredWorkspace(params.name);
    return json(
      await migrateService(workspace.serviceName, payload.sourceNodeId, payload.targetNodeId),
    );
  } catch (error) {
    return json(
      { error: error instanceof Error ? error.message : "Failed to migrate service." },
      { status: 400 },
    );
  }
};
