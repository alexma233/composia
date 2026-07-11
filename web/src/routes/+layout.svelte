<script lang="ts">
  import { onMount, untrack } from "svelte";
  import type { Snippet } from "svelte";

  import type { LayoutData } from "./$types";

  import "../app.css";

  import { createI18nContext } from "$lib/i18n";
  import { initializePreferences } from "$lib/preferences";

  interface Props {
    data: LayoutData;
    children?: Snippet;
  }

  let { data, children }: Props = $props();
  let appReady = $state(false);
  const { setLocale } = createI18nContext(
    untrack(() => data.locale),
    untrack(() => data.dictionary),
  );

  onMount(() => {
    appReady = true;
    return initializePreferences((locale) => void setLocale(locale));
  });
</script>

<div data-app-ready={appReady ? "" : undefined}>
  {@render children?.()}
</div>
