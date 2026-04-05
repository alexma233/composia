import type { Actions, PageServerLoad } from "./$types";

import { fail, redirect } from "@sveltejs/kit";

import {
  controllerConfig,
  loadRepoHead,
  loadServiceDetail,
  loadServiceSecret,
  updateServiceSecret,
} from "$lib/server/controller";
import { loadServiceWorkspace } from "$lib/server/service-index";

export const actions: Actions = {
  save: async ({ request, params }) => {
    const form = await request.formData();
    const content = String(form.get("content") ?? "");
    const baseRevision = String(form.get("baseRevision") ?? "");
    const commitMessage = String(form.get("commitMessage") ?? "");

    if (!baseRevision) {
      return fail(400, {
        error: "Missing base revision.",
        content,
        commitMessage,
      });
    }

    try {
      const workspace = await loadServiceWorkspace(params.name);
      if (!workspace?.isDeclared || !workspace.serviceName) {
        return fail(400, {
          error:
            "Add a valid composia-meta.yaml for this folder before editing secrets.",
          content,
          commitMessage,
        });
      }
      await updateServiceSecret(
        workspace.serviceName,
        content,
        baseRevision,
        commitMessage,
      );
    } catch (error) {
      return fail(500, {
        error:
          error instanceof Error
            ? error.message
            : "Failed to update service secret.",
        content,
        commitMessage,
      });
    }

    throw redirect(303, `/services/${params.name}/secret`);
  },
};

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      service: null,
      workspace: null,
      secret: null,
      head: null,
    };
  }

  try {
    const workspace = await loadServiceWorkspace(params.name);
    if (!workspace?.isDeclared || !workspace.serviceName) {
      return {
        ready: true,
        error:
          "Add a valid composia-meta.yaml for this folder before editing secrets.",
        service: null,
        workspace,
        secret: null,
        head: null,
      };
    }

    const [service, secret, head] = await Promise.all([
      loadServiceDetail(workspace.serviceName),
      loadServiceSecret(workspace.serviceName),
      loadRepoHead(),
    ]);

    return {
      ready: true,
      error: null,
      service,
      workspace,
      secret,
      head,
    };
  } catch (error) {
    return {
      ready: true,
      error:
        error instanceof Error
          ? error.message
          : "Failed to load service secret.",
      service: null,
      workspace: null,
      secret: null,
      head: null,
    };
  }
};
