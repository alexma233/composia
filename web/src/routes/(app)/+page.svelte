<script lang="ts">
  import type { PageData } from "./$types";

  import DashboardContent from "./dashboard-content.svelte";
  import DashboardSkeleton from "$lib/components/app/dashboard-skeleton.svelte";
  import { getMessages } from "$lib/i18n";

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();
  const messages = getMessages();
</script>

<svelte:head>
  <title>{$messages.dashboard.title} - {$messages.app.name}</title>
  <meta name="description" content={$messages.dashboard.pageDescription} />
</svelte:head>

{#await data.content}
  <DashboardSkeleton label={$messages.common.loadingWithDots} />
{:then content}
  <DashboardContent data={content} />
{/await}
