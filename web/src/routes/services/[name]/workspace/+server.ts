import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import {
  jsonError,
  loadServiceWorkspaceSummary,
} from "$lib/server/service-workspace-route";
import { loadServiceWorkspaceFile } from "$lib/server/service-workspace";
import { normalizeServiceRelativePath } from "$lib/service-workspace";

export const GET: RequestHandler = async ({ params, url }) => {
  try {
    const summary = await loadServiceWorkspaceSummary(params.name);
    const path = url.searchParams.get("path");
    const normalizedPath = path ? normalizeServiceRelativePath(path) : "";
    const file = normalizedPath
      ? await loadServiceWorkspaceFile(
          summary.workspace.serviceName || null,
          params.name,
          normalizedPath,
        )
      : null;

    return json({ ...summary, file });
  } catch (error) {
    if (error instanceof Response) {
      return error;
    }
    return jsonError(error, "Failed to load service workspace.");
  }
};
