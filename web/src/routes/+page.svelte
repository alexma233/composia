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
  import ServiceCard from '$lib/components/app/service-card.svelte';
  import NodeCard from '$lib/components/app/node-card.svelte';

  interface Props {
    data: PageData;
    children?: Snippet;
  }

	let { data }: Props = $props();

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
  <Card>
    <CardHeader>
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
    </CardHeader>
    <CardContent>
    <section class="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
		<Card>
        <CardHeader>
          <div class="flex items-center justify-between gap-3">
            <CardTitle class="section-title" level="2">
              <a class="hover:text-foreground/80 transition-colors" href="/services">{$messages.dashboard.services}</a>
            </CardTitle>
            <Badge variant="outline">{data.dashboard?.services.length ?? 0}</Badge>
          </div>
        </CardHeader>
        <CardContent>
          <div class="space-y-3">
            {#if data.dashboard?.services.length}
              {#each data.dashboard.services as service}
                <ServiceCard {service} />
              {/each}
            {:else}
              <div class="empty-state">{$messages.common.noData}</div>
            {/if}
          </div>
        </CardContent>
      </Card>

      <div class="grid gap-6">
			<Card>
          <CardHeader>
            <div class="flex items-center justify-between gap-3">
              <CardTitle class="section-title" level="2">
                <a class="hover:text-foreground/80 transition-colors" href="/nodes">{$messages.dashboard.nodes}</a>
              </CardTitle>
              <Badge variant="outline">{data.dashboard?.nodes.length ?? 0}</Badge>
            </div>
          </CardHeader>
          <CardContent>
            <div class="space-y-3">
              {#if data.dashboard?.nodes.length}
                {#each data.dashboard.nodes as node}
                  <NodeCard {node} />
                {/each}
              {:else}
                <div class="empty-state">{$messages.common.noData}</div>
              {/if}
            </div>
          </CardContent>
        </Card>

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
    </section>
    </CardContent>
  </Card>
</div>
