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
    formatTimestamp,
    isTaskRecent,
    onlineStatusTone,
    runtimeStatusLabel,
    runtimeStatusTone,
  } from "$lib/presenters";
  import { messages } from '$lib/i18n';
  import TaskItem from '$lib/components/app/task-item.svelte';

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
          <CardTitle class="page-title">{$messages.dashboard.title}</CardTitle>
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
            <CardTitle class="section-title">
              <a class="hover:text-foreground/80 transition-colors" href="/services">{$messages.dashboard.services}</a>
            </CardTitle>
            <Badge variant="outline">{data.dashboard?.services.length ?? 0}</Badge>
          </div>
        </CardHeader>
        <CardContent>
          <div class="space-y-3">
            {#if data.dashboard?.services.length}
              {#each data.dashboard.services as service}
                <a
                  href={`/services/${service.folder ?? service.name}`}
                  class="list-row"
                >
                  <div class="flex flex-wrap items-center justify-between gap-3">
                    <div class="min-w-0 flex-1">
                      <div class="truncate text-sm font-medium">{service.name}</div>
                      <div class="truncate text-xs text-muted-foreground">
                        {$messages.dashboard.updated} {formatTimestamp(service.updatedAt)}
                      </div>
                    </div>
                    <Badge variant={runtimeStatusTone(service.runtimeStatus)}>
                      {runtimeStatusLabel(service.runtimeStatus, $messages)}
                    </Badge>
                  </div>
                </a>
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
              <CardTitle class="section-title">
                <a class="hover:text-foreground/80 transition-colors" href="/nodes">{$messages.dashboard.nodes}</a>
              </CardTitle>
              <Badge variant="outline">{data.dashboard?.nodes.length ?? 0}</Badge>
            </div>
          </CardHeader>
          <CardContent>
            <div class="space-y-3">
              {#if data.dashboard?.nodes.length}
                {#each data.dashboard.nodes as node}
                  <a
                    href={`/nodes/${node.nodeId}`}
                    class="list-row"
                  >
                    <div class="flex flex-wrap items-center justify-between gap-3">
                      <div class="min-w-0 flex-1">
                        <div class="truncate text-sm font-medium">{node.displayName}</div>
                        <div class="truncate text-xs text-muted-foreground">{node.nodeId}</div>
                      </div>
                      <Badge variant={onlineStatusTone(node.isOnline)}>
                        {node.isOnline ? $messages.status.online : $messages.status.offline}
                      </Badge>
                    </div>
                    <div class="mt-2 text-xs text-muted-foreground">
                      {$messages.dashboard.lastHeartbeat} {formatTimestamp(node.lastHeartbeat)}
                    </div>
                  </a>
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
              <CardTitle class="section-title">
                <a class="hover:text-foreground/80 transition-colors" href="/tasks">{$messages.dashboard.tasks}</a>
              </CardTitle>
              <Badge variant="outline">{totalTaskCount()}</Badge>
            </div>
          </CardHeader>
          <CardContent>
            <div class="space-y-3">
              {#if recentTasks.length}
                {#each recentTasks as task}
                  <TaskItem {task} showService />
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
