<script lang="ts">
  import type { PageData } from "./$types";

  import ListPageSkeleton from "$lib/components/app/list-page-skeleton.svelte";
  import { getMessages } from "$lib/i18n";
  import TasksContent from "./tasks-content.svelte";

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();
  const messages = getMessages();
</script>

<svelte:head>
  <title>{$messages.tasks.title} - {$messages.app.name}</title>
  <meta name="description" content={$messages.tasks.pageDescription} />
</svelte:head>

{#await data.content}
  <ListPageSkeleton label={$messages.common.loadingWithDots} columns={5} />
{:then content}
  <TasksContent data={content} />
{/await}
