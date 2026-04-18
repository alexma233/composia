import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { runServiceAction, type ServiceAction } from "$lib/server/controller";
import {
  jsonCapabilityError,
  jsonControllerError,
} from "$lib/server/controller-route";
import { requireDeclaredWorkspace } from "$lib/server/service-workspace-route";

export const POST: RequestHandler = async ({ params }) => {
  try {
    const workspace = await requireDeclaredWorkspace(params.name);

    if (!isServiceAction(params.action)) {
      return json({ error: "Unsupported service action." }, { status: 400 });
    }

    const guardedAction = serviceCapabilityAction(params.action);
    if (guardedAction) {
      const capability = workspace.actions[guardedAction];
      if (!capability.enabled) {
        return jsonCapabilityError(
          capability.reasonCode,
          "Service action is unavailable.",
        );
      }
    }

    return json(await runServiceAction(workspace.serviceName, params.action));
  } catch (error) {
    return jsonControllerError(error, "Failed to run action.", {}, 400);
  }
};

function serviceCapabilityAction(
  action: ServiceAction,
): "backup" | "dnsUpdate" | "caddySync" | null {
  switch (action) {
    case "backup":
      return "backup";
    case "dns_update":
      return "dnsUpdate";
    case "caddy_sync":
      return "caddySync";
    default:
      return null;
  }
}

function isServiceAction(action: string): action is ServiceAction {
  return (
    action === "deploy" ||
    action === "update" ||
    action === "stop" ||
    action === "restart" ||
    action === "backup" ||
    action === "dns_update" ||
    action === "caddy_sync"
  );
}
