import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { normalizeServiceRelativePath } from "$lib/service-workspace";
import { loadServiceWorkspace } from "$lib/server/service-index";
import {
  loadServiceWorkspaceFile,
  saveServiceWorkspaceFile,
} from "$lib/server/service-workspace";

export const GET: RequestHandler = async ({ params, url }) => {
  const path = url.searchParams.get("path") ?? "";

  try {
    const workspace = await loadServiceWorkspace(params.name);
    const file = await loadServiceWorkspaceFile(
      workspace?.serviceName ?? null,
      params.name,
      normalizeServiceRelativePath(path),
    );
    return json({ file });
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to load workspace file.",
      },
      { status: 400 },
    );
  }
};

export const POST: RequestHandler = async ({ params, request }) => {
  try {
    const workspace = await loadServiceWorkspace(params.name);
    if (!workspace?.serviceName) {
      return json(
        { error: "Service is not declared. Add composia-meta.yaml before editing files." },
        { status: 400 },
      );
    }

    const payload = (await request.json()) as {
      path?: string;
      content?: string;
      baseRevision?: string;
    };

    if (!payload.path || !payload.baseRevision) {
      return json(
        { error: "Path and base revision are required." },
        { status: 400 },
      );
    }

    const result = await saveServiceWorkspaceFile(
      workspace.serviceName,
      params.name,
      normalizeServiceRelativePath(payload.path),
      payload.content ?? "",
      payload.baseRevision,
    );

    return json({ file: result.file, write: result.write });
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to save workspace file.",
      },
      { status: 400 },
    );
  }
};
