<script lang="ts">
  import { invalidateAll } from '$app/navigation';
  import { onMount } from 'svelte';
  import type { PageData } from './$types';
  import type { Snippet } from 'svelte';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { startPolling } from '$lib/refresh';
  import {
    isTaskRecent,
  } from "$lib/presenters";
  import { messages } from '$lib/i18n';
  import TaskCard from '$lib/components/app/task-card.svelte';

  interface Props {
    data: PageData;
    children?: Snippet;
  }

	let { data }: Props = $props();

  let runningServiceCount = $derived(data.dashboard?.system.runningServiceCount ?? 0);
  let totalServiceCount = $derived(data.dashboard?.system.serviceCount ?? 0);
  let onlineNodeCount = $derived(data.dashboard?.system.onlineNodeCount ?? 0);
  let configuredNodeCount = $derived(data.dashboard?.system.configuredNodeCount ?? 0);

	let recentTasks = $derived((data.dashboard?.tasks ?? [])
		.filter((t) => isTaskRecent(t.createdAt))
		.slice(0, 6));

  function totalTaskCount() {
    return 'totalTaskCount' in data ? data.totalTaskCount : 0;
  }

  onMount(() => startPolling(() => invalidateAll(), { intervalMs: 5000 }));
</script>

<svelte:head>
  <title>{$messages.dashboard.title} - {$messages.app.name}</title>
  <meta
    name="description"
    content={$messages.dashboard.pageDescription}
  />
</svelte:head>

<div class="page-shell">
  <div class="page-stack">
    <div class="page-header">
      <div class="page-heading">
        <CardTitle class="page-title" level="1">{$messages.dashboard.title}</CardTitle>
      </div>
    </div>

    {#if data.error}
      <Alert variant="destructive">
        <AlertTitle>{$messages.error.loadFailed}</AlertTitle>
        <AlertDescription>{data.error}</AlertDescription>
      </Alert>
    {/if}

    <div class="grid gap-4 sm:grid-cols-2">
      <a href="/services" class="no-underline">
        <Card class="transition-colors hover:bg-accent/50">
          <CardHeader class="pb-2">
            <CardTitle class="text-sm font-medium text-muted-foreground">
              {$messages.dashboard.services}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div class="text-3xl font-bold tabular-nums">{runningServiceCount}</div>
            <p class="text-xs text-muted-foreground mt-1">
              {$messages.dashboard.serviceStatRunning
                .replace('{running}', String(runningServiceCount))
                .replace('{total}', String(totalServiceCount))}
            </p>
          </CardContent>
        </Card>
      </a>

      <a href="/nodes" class="no-underline">
        <Card class="transition-colors hover:bg-accent/50">
          <CardHeader class="pb-2">
            <CardTitle class="text-sm font-medium text-muted-foreground">
              {$messages.dashboard.nodes}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div class="text-3xl font-bold tabular-nums">{onlineNodeCount}</div>
            <p class="text-xs text-muted-foreground mt-1">
              {$messages.dashboard.nodeStatOnline
                .replace('{online}', String(onlineNodeCount))
                .replace('{total}', String(configuredNodeCount))}
            </p>
          </CardContent>
        </Card>
      </a>
    </div>

    <Card>
      <CardHeader>
        <div class="flex items-center justify-between gap-3">
          <CardTitle class="section-title" level="2">
            <a class="hover:text-foreground/80 transition-colors" href="/tasks">{$messages.dashboard.tasks}</a>
          </CardTitle>
          <Badge variant="outline">{totalTaskCount()}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        <div class="space-y-3">
          {#if recentTasks.length}
            {#each recentTasks as task}
              <TaskCard {task} showService />
            {/each}
          {:else}
            <div class="empty-state">{$messages.dashboard.last24Hours}</div>
          {/if}
        </div>
      </CardContent>
    </Card>
  </div>
</div>
