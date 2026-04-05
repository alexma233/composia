import type { Actions, PageServerLoad } from './$types';

import { fail, redirect } from '@sveltejs/kit';

import {
  controllerConfig,
  loadRepoHead,
  loadServiceDetail,
  loadServiceSecret,
  updateServiceSecret
} from '$lib/server/controller';

export const actions: Actions = {
  save: async ({ request, params }) => {
    const form = await request.formData();
    const content = String(form.get('content') ?? '');
    const baseRevision = String(form.get('baseRevision') ?? '');
    const commitMessage = String(form.get('commitMessage') ?? '');

    if (!baseRevision) {
      return fail(400, { error: 'Missing base revision.', content, commitMessage });
    }

    try {
      await updateServiceSecret(params.name, content, baseRevision, commitMessage);
    } catch (error) {
      return fail(500, {
        error: error instanceof Error ? error.message : 'Failed to update service secret.',
        content,
        commitMessage
      });
    }

    throw redirect(303, `/services/${params.name}/secret`);
  }
};

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, service: null, secret: null, head: null };
  }

  try {
    const [service, secret, head] = await Promise.all([
      loadServiceDetail(params.name),
      loadServiceSecret(params.name),
      loadRepoHead()
    ]);

    return {
      ready: true,
      error: null,
      service,
      secret,
      head
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : 'Failed to load service secret.',
      service: null,
      secret: null,
      head: null
    };
  }
};
