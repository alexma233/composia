<script lang="ts">
  import type { PageData } from "./$types";

  import BackupsContent from "./backups-content.svelte";
  import ListPageSkeleton from "$lib/components/app/list-page-skeleton.svelte";
  import { getMessages } from "$lib/i18n";

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();
  const messages = getMessages();
</script>

<svelte:head>
  <title>{$messages.backups.title} - {$messages.app.name}</title>
  <meta name="description" content={$messages.backups.pageDescription} />
</svelte:head>

{#await data.content}
  <ListPageSkeleton label={$messages.common.loadingWithDots} columns={5} />
{:then content}
  <BackupsContent data={content} />
{/await}
