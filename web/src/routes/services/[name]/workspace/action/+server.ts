import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import {
  backupService,
  deployService,
  restartService,
  stopService,
  updateService,
  updateServiceDNS,
} from "$lib/server/controller";
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

    switch (payload.action) {
      case "deploy":
        return json(await deployService(workspace.serviceName));
      case "update":
        return json(await updateService(workspace.serviceName));
      case "stop":
        return json(await stopService(workspace.serviceName));
      case "restart":
        return json(await restartService(workspace.serviceName));
      case "backup":
        return json(await backupService(workspace.serviceName));
      case "dns_update":
        return json(await updateServiceDNS(workspace.serviceName));
      default:
        return json({ error: "Unsupported service action." }, { status: 400 });
    }
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
