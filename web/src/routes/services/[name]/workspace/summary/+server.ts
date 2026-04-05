import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

import { loadServiceBackups, loadServiceTasks } from '$lib/server/controller';
import { loadServiceWorkspace } from '$lib/server/service-index';

export const GET: RequestHandler = async ({ params }) => {
  try {
    const workspace = await loadServiceWorkspace(params.name);
    if (!workspace) {
      return json({ error: 'Service folder not found.' }, { status: 404 });
    }

    const [tasks, backups] = await Promise.all([
      workspace.isDeclared && workspace.serviceName ? loadServiceTasks(workspace.serviceName) : Promise.resolve([]),
      workspace.isDeclared && workspace.serviceName ? loadServiceBackups(workspace.serviceName) : Promise.resolve([])
    ]);

    return json({ workspace, tasks, backups });
  } catch (error) {
    return json(
      { error: error instanceof Error ? error.message : 'Failed to load service summary.' },
      { status: 400 }
    );
  }
};
