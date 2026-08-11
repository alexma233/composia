<script lang="ts">
  import type { ActionData, PageData } from "./$types";

  import ListPageSkeleton from "$lib/components/app/list-page-skeleton.svelte";
  import { getMessages } from "$lib/i18n";
  import ServicesContent from "./services-content.svelte";

  interface Props {
    data: PageData;
    form: ActionData;
  }

  let { data, form }: Props = $props();
  const messages = getMessages();
</script>

<svelte:head>
  <title>{$messages.services.title} - {$messages.app.name}</title>
  <meta name="description" content={$messages.services.pageDescription} />
</svelte:head>

{#await data.content}
  <ListPageSkeleton label={$messages.common.loadingWithDots} columns={5} />
{:then content}
  <ServicesContent data={content} {form} />
{/await}
