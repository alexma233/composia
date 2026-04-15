import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { loadServiceWorkspaceSummary } from "$lib/server/service-workspace-route";
import {
  deleteServiceWorkspacePath,
  moveServiceWorkspacePath,
} from "$lib/server/service-workspace";
import { normalizeServiceRelativePath } from "$lib/service-workspace";

export const PATCH: RequestHandler = async ({ params, request }) => {
  try {
    const payload = (await request.json()) as {
      sourcePath?: string;
      destinationPath?: string;
      baseRevision?: string;
    };

    if (
      !payload.sourcePath ||
      !payload.destinationPath ||
      !payload.baseRevision
    ) {
      return json(
        {
          error:
            "Source path, destination path, and base revision are required.",
        },
        { status: 400 },
      );
    }

    const write = await moveServiceWorkspacePath(
      params.name,
      normalizeServiceRelativePath(payload.sourcePath),
      normalizeServiceRelativePath(payload.destinationPath),
      payload.baseRevision,
    );
    const { workspace, fileTree } = await loadServiceWorkspaceSummary(
      params.name,
    );
    return json({ write, workspace, fileTree });
  } catch (error) {
    return json(
      {
        error: error instanceof Error ? error.message : "Failed to move path.",
      },
      { status: 400 },
    );
  }
};

export const DELETE: RequestHandler = async ({ params, request }) => {
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

    const write = await deleteServiceWorkspacePath(
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
          error instanceof Error ? error.message : "Failed to delete path.",
      },
      { status: 400 },
    );
  }
};
