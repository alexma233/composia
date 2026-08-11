import type { PageServerLoad } from "./$types";
import type { Actions } from "./$types";

import { fail, redirect } from "@sveltejs/kit";

import {
  controllerConfig,
  loadRepoHead,
  updateRepoFile,
} from "$lib/server/controller";
import { normalizeServiceRelativePath } from "$lib/service-workspace";
import { loadServiceWorkspaces } from "$lib/server/service-index";
import { isDocumentRequest } from "$lib/server/request";

export const load: PageServerLoad = async ({ depends, request }) => {
  depends("app:services");
  const config = controllerConfig();
  if (!config.ready) {
    return {
      content: {
        ready: false,
        error: config.reason,
        services: [],
        repoHead: null,
      },
    };
  }

  const content = Promise.all([loadServiceWorkspaces(), loadRepoHead()])
    .then(([services, repoHead]) => ({
      ready: true,
      error: null,
      services,
      repoHead,
    }))
    .catch((error: unknown) => ({
      ready: true,
      error:
        error instanceof Error ? error.message : "Failed to load services.",
      services: [],
      repoHead: null,
    }));

  return { content: isDocumentRequest(request) ? content : await content };
};

export const actions: Actions = {
  create: async ({ request }) => {
    const form = await request.formData();
    const folderInput = String(form.get("folder") ?? "");
    const baseRevision = String(form.get("baseRevision") ?? "");

    if (!baseRevision) {
      return fail(400, {
        error: "Missing base revision.",
        folder: folderInput,
      });
    }

    let folder = "";
    try {
      folder = normalizeServiceRelativePath(folderInput);
    } catch (error) {
      return fail(400, {
        error: error instanceof Error ? error.message : "Invalid folder name.",
        folder: folderInput,
      });
    }

    if (!folder || folder.includes("/")) {
      return fail(400, {
        error: "New services must be created as a single top-level folder.",
        folder: folderInput,
      });
    }

    try {
      const composeWrite = await updateRepoFile(
        `${folder}/docker-compose.yaml`,
        "",
        baseRevision,
      );
      await updateRepoFile(
        `${folder}/composia-meta.yaml`,
        "",
        composeWrite.commitId,
      );
    } catch (error) {
      return fail(400, {
        error:
          error instanceof Error
            ? error.message
            : "Failed to create service files.",
        folder: folderInput,
      });
    }

    throw redirect(303, `/services/${folder}`);
  },
};
