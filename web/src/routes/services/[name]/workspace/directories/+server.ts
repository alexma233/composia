import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { loadServiceWorkspaceSummary } from "$lib/server/service-workspace-route";
import { createServiceWorkspaceDirectory } from "$lib/server/service-workspace";
import { normalizeServiceRelativePath } from "$lib/service-workspace";

export const POST: RequestHandler = async ({ params, request }) => {
  try {
    const payload = (await request.json()) as {
      path?: string;
      baseRevision?: string;
    };

    if (!payload.path || !payload.baseRevision) {
      return json(
        { error: "Path and base revision are required." },
        { status: 400 },
      );
    }

    const write = await createServiceWorkspaceDirectory(
      params.name,
      normalizeServiceRelativePath(payload.path),
      payload.baseRevision,
    );
    const { workspace, fileTree } = await loadServiceWorkspaceSummary(
      params.name,
    );
    return json({ write, workspace, fileTree });
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to create directory.",
      },
      { status: 400 },
    );
  }
};
