import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { loadServiceWorkspace } from "$lib/server/service-index";
import {
  createServiceWorkspaceDirectory,
  deleteServiceWorkspacePath,
  loadServiceFileTree,
  moveServiceWorkspacePath,
} from "$lib/server/service-workspace";
import { normalizeServiceRelativePath } from "$lib/service-workspace";

export const POST: RequestHandler = async ({ params, request }) => {
  try {
    const payload = (await request.json()) as {
      action?: "create_directory" | "move" | "delete";
      path?: string;
      sourcePath?: string;
      destinationPath?: string;
      baseRevision?: string;
    };

    if (!payload.baseRevision) {
      return json({ error: "Base revision is required." }, { status: 400 });
    }

    let write;
    switch (payload.action) {
      case "create_directory":
        if (!payload.path) {
          return json({ error: "Path is required." }, { status: 400 });
        }
        write = await createServiceWorkspaceDirectory(
          params.name,
          normalizeServiceRelativePath(payload.path),
          payload.baseRevision,
        );
        break;
      case "move":
        if (!payload.sourcePath || !payload.destinationPath) {
          return json(
            { error: "Source and destination paths are required." },
            { status: 400 },
          );
        }
        write = await moveServiceWorkspacePath(
          params.name,
          normalizeServiceRelativePath(payload.sourcePath),
          normalizeServiceRelativePath(payload.destinationPath),
          payload.baseRevision,
        );
        break;
      case "delete":
        if (!payload.path) {
          return json({ error: "Path is required." }, { status: 400 });
        }
        write = await deleteServiceWorkspacePath(
          params.name,
          normalizeServiceRelativePath(payload.path),
          payload.baseRevision,
        );
        break;
      default:
        return json(
          { error: "Unsupported file management action." },
          { status: 400 },
        );
    }

    const [workspace, fileTree] = await Promise.all([
      loadServiceWorkspace(params.name),
      loadServiceFileTree(params.name),
    ]);

    return json({ write, workspace, fileTree });
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to update workspace files.",
      },
      { status: 400 },
    );
  }
};
