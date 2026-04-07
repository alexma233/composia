import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { runServiceAction, type ServiceAction } from "$lib/server/controller";
import { loadServiceWorkspace } from "$lib/server/service-index";

export const POST: RequestHandler = async ({ params, request }) => {
  try {
    const payload = (await request.json()) as { action?: string };
    const workspace = await loadServiceWorkspace(params.name);
    if (!workspace) {
      return json({ error: "Service folder not found." }, { status: 404 });
    }
    if (!workspace.isDeclared || !workspace.serviceName) {
      return json(
        {
          error:
            "Add a valid composia-meta.yaml for this folder before running service actions.",
        },
        { status: 400 },
      );
    }

    if (!isServiceAction(payload.action)) {
      return json({ error: "Unsupported service action." }, { status: 400 });
    }

    return json(await runServiceAction(workspace.serviceName, payload.action));
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to trigger service action.",
      },
      { status: 400 },
    );
  }
};

function isServiceAction(action: string | undefined): action is ServiceAction {
  return (
    action === "deploy" ||
    action === "update" ||
    action === "stop" ||
    action === "restart" ||
    action === "backup" ||
    action === "dns_update"
  );
}
