import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { deleteRepoPath, moveRepoPath } from "$lib/server/controller";
import { normalizeServiceRelativePath } from "$lib/service-workspace";

export const POST: RequestHandler = async ({ params, request }) => {
  try {
    const payload = (await request.json()) as {
      action?: "rename" | "delete";
      folder?: string;
      baseRevision?: string;
    };

    if (!payload.baseRevision) {
      return json({ error: "Base revision is required." }, { status: 400 });
    }

    if (payload.action === "rename") {
      const folder = normalizeServiceRootFolder(payload.folder ?? "");
      const write = await moveRepoPath(
        params.name,
        folder,
        payload.baseRevision,
      );
      return json({ write, redirectTo: `/services/${folder}` });
    }

    if (payload.action === "delete") {
      const write = await deleteRepoPath(params.name, payload.baseRevision);
      return json({ write, redirectTo: "/services" });
    }

    return json({ error: "Unsupported service root action." }, { status: 400 });
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to update service folder.",
      },
      { status: 400 },
    );
  }
};

function normalizeServiceRootFolder(input: string) {
  const folder = normalizeServiceRelativePath(input);
  if (!folder || folder.includes("/")) {
    throw new Error("Service folder must stay a single top-level directory.");
  }
  return folder;
}
