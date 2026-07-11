import type { LayoutServerLoad } from "./$types";

import { normalizeLocale } from "$lib/i18n/locales";

export const load: LayoutServerLoad = async ({ cookies, locals }) => ({
  locale: normalizeLocale(cookies.get("composia.locale")),
  user: locals.user,
});
