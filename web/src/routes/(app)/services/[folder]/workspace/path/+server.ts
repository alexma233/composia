import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { loadServiceWorkspaceFiles } from "$lib/server/service-workspace-route";
import {
  deleteServiceWorkspacePath,
  moveServiceWorkspacePath,
} from "$lib/server/service-workspace";
import { jsonApiError } from "$lib/server/controller-route";
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
      return jsonApiError("SOURCE_DEST_REVISION_REQUIRED");
    }

    const write = await moveServiceWorkspacePath(
      params.folder,
      normalizeServiceRelativePath(payload.sourcePath),
      normalizeServiceRelativePath(payload.destinationPath),
      payload.baseRevision,
    );
    const { workspace, fileTree } = await loadServiceWorkspaceFiles(
      params.folder,
    );
    return json({ write, workspace, fileTree });
  } catch (error) {
    return json(
      {
        error: error instanceof Error ? error.message : "Failed to move path.",
        errorCode: "MOVE_PATH_FAILED",
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
      return jsonApiError("PATH_REVISION_REQUIRED");
    }

    const write = await deleteServiceWorkspacePath(
      params.folder,
      normalizeServiceRelativePath(payload.path),
      payload.baseRevision,
    );
    const { workspace, fileTree } = await loadServiceWorkspaceFiles(
      params.folder,
    );
    return json({ write, workspace, fileTree });
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error ? error.message : "Failed to delete path.",
        errorCode: "DELETE_PATH_FAILED",
      },
      { status: 400 },
    );
  }
};
