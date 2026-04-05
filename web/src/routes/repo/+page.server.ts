import type { PageServerLoad } from './$types';
import { fail, redirect } from '@sveltejs/kit';

import {
  controllerConfig,
  loadRepoEntries,
  loadRepoFile,
  loadRepoHead,
  syncRepo,
  updateRepoFile
} from '$lib/server/controller';

export const actions = {
	sync: async ({ url }) => {
		try {
			await syncRepo();
		} catch (error) {
			return fail(500, {
				error: error instanceof Error ? error.message : 'Failed to sync repo.'
			});
		}

		throw redirect(303, url.pathname + url.search);
	},
  save: async ({ request, url }) => {
    const form = await request.formData();
    const path = String(form.get('path') ?? '');
    const content = String(form.get('content') ?? '');
    const baseRevision = String(form.get('baseRevision') ?? '');
    const commitMessage = String(form.get('commitMessage') ?? '');

    if (!path || !baseRevision) {
      return fail(400, { error: 'Missing path or base revision.' });
    }

    try {
      await updateRepoFile(path, content, baseRevision, commitMessage);
    } catch (error) {
      return fail(500, {
        error: error instanceof Error ? error.message : 'Failed to update repo file.',
        path,
        content,
        commitMessage
      });
    }

    const next = new URL(url);
    next.searchParams.set('file', path);
    throw redirect(303, next.pathname + next.search);
  }
};

export const load: PageServerLoad = async ({ url }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, head: null, entries: [], file: null, path: '' };
  }

  const path = url.searchParams.get('path') ?? '';
  const filePath = url.searchParams.get('file') ?? '';

  try {
    const [head, entries, file] = await Promise.all([
      loadRepoHead(),
      loadRepoEntries(path),
      filePath ? loadRepoFile(filePath) : Promise.resolve(null)
    ]);

    return {
      ready: true,
      error: null,
      head,
      entries,
      file,
      path
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : 'Failed to load repo data.',
      head: null,
      entries: [],
      file: null,
      path
    };
  }
};
