import type { PageServerLoad } from "./$types";

import { controllerConfig, loadDashboard } from "$lib/server/controller";
import { isDocumentRequest } from "$lib/server/request";

export const load: PageServerLoad = async ({ depends, request }) => {
  depends("app:dashboard");
  const config = controllerConfig();
  if (!config.ready) {
    return {
      content: {
        ready: false,
        error: config.reason,
        dashboard: null,
        totalTaskCount: 0,
        totalBackupCount: 0,
      },
    };
  }

  const content = loadDashboard()
    .then((dashboard) => ({
      ready: true,
      error: null,
      dashboard,
      totalTaskCount: dashboard.totalTaskCount,
      totalBackupCount: dashboard.totalBackupCount,
    }))
    .catch((error: unknown) => ({
      ready: true,
      error:
        error instanceof Error
          ? error.message
          : "Failed to load dashboard data.",
      dashboard: null,
      totalTaskCount: 0,
      totalBackupCount: 0,
    }));

  return { content: isDocumentRequest(request) ? content : await content };
};
