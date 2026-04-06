import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { loadBackups, loadTasks } from "$lib/server/controller";
import { loadServiceWorkspace } from "$lib/server/service-index";

export const GET: RequestHandler = async ({ params }) => {
  try {
    const workspace = await loadServiceWorkspace(params.name);
    if (!workspace) {
      return json({ error: "Service folder not found." }, { status: 404 });
    }

    const [tasksResult, backupsResult] = await Promise.all([
      workspace.isDeclared && workspace.serviceName
        ? loadTasks(1, 20, { serviceName: workspace.serviceName })
        : Promise.resolve({ items: [], totalCount: 0 }),
      workspace.isDeclared && workspace.serviceName
        ? loadBackups(1, 20, { serviceName: workspace.serviceName })
        : Promise.resolve({ items: [], totalCount: 0 }),
    ]);

    return json({ workspace, tasks: tasksResult.items, backups: backupsResult.items });
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to load service summary.",
      },
      { status: 400 },
    );
  }
};
