import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import {
  runServiceAction,
  syncNodeCaddyFiles,
  type ServiceAction,
} from "$lib/server/controller";
import { jsonControllerError } from "$lib/server/controller-route";
import { requireDeclaredWorkspace } from "$lib/server/service-workspace-route";

export const POST: RequestHandler = async ({ params }) => {
  try {
    const workspace = await requireDeclaredWorkspace(params.name);

    if (params.action === "caddy-sync") {
      return json(
        await syncNodeCaddyFiles(workspace.node, {
          serviceName: workspace.serviceName,
        }),
      );
    }

    if (!isServiceAction(params.action)) {
      return json({ error: "Unsupported service action." }, { status: 400 });
    }

    return json(await runServiceAction(workspace.serviceName, params.action));
  } catch (error) {
    return jsonControllerError(error, "Failed to run action.", {}, 400);
  }
};

function isServiceAction(action: string): action is ServiceAction {
  return (
    action === "deploy" ||
    action === "update" ||
    action === "stop" ||
    action === "restart" ||
    action === "backup" ||
    action === "dns_update"
  );
}
