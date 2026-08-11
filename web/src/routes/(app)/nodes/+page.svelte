<script lang="ts">
  import type { PageData } from "./$types";

  import ListPageSkeleton from "$lib/components/app/list-page-skeleton.svelte";
  import { getMessages } from "$lib/i18n";
  import NodesContent from "./nodes-content.svelte";

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();
  const messages = getMessages();
</script>

<svelte:head>
  <title>{$messages.nodes.title} - {$messages.app.name}</title>
  <meta name="description" content={$messages.nodes.pageDescription} />
</svelte:head>

{#await data.content}
  <ListPageSkeleton label={$messages.common.loadingWithDots} columns={3} />
{:then content}
  <NodesContent data={content} />
{/await}
