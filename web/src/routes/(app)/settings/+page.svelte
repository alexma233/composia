<script lang="ts">
  import type { PageData } from "./$types";

  import ListPageSkeleton from "$lib/components/app/list-page-skeleton.svelte";
  import { getMessages } from "$lib/i18n";
  import SettingsContent from "./settings-content.svelte";

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();
  const messages = getMessages();
</script>

<svelte:head>
  <title>{$messages.settings.title} - {$messages.app.name}</title>
</svelte:head>

{#await data.content}
  <ListPageSkeleton label={$messages.common.loadingWithDots} columns={3} />
{:then content}
  <SettingsContent data={content} />
{/await}
