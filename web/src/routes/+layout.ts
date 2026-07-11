import type { LayoutLoad } from "./$types";

import { loadDictionary } from "$lib/i18n";

export const load: LayoutLoad = async ({ data }) => ({
  ...data,
  dictionary: await loadDictionary(data.locale),
});
