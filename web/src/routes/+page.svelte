<script lang="ts">
  import type { PageData } from './$types';
  import type { Snippet } from 'svelte';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { formatTimestamp, onlineStatusTone, runtimeStatusLabel, runtimeStatusTone } from '$lib/presenters';
  import { messages } from '$lib/i18n';
  import TaskItem from '$lib/components/app/task-item.svelte';

  interface Props {
    data: PageData;
    children?: Snippet;
  }

  let { data }: Props = $props();

  function isTaskRecent(createdAt: string) {
    const createdAtMs = Date.parse(createdAt);
    if (Number.isNaN(createdAtMs)) return false;
    return Date.now() - createdAtMs <= 24 * 60 * 60 * 1000;
  }

  let recentTasks = $derived((data.dashboard?.tasks ?? [])
    .filter((t) => isTaskRecent(t.createdAt))
    .slice(0, 6));

  function totalTaskCount() {
    return 'totalTaskCount' in data ? data.totalTaskCount : 0;
  }
</script>

<svelte:head>
  <title>Composia Control Plane</title>
  <meta
    name="description"
    content="Composia controller overview with live services, nodes, and task history."
  />
</svelte:head>

<div class="page-shell">
  <div class="page-stack">
    {#if data.error}
      <Alert variant="destructive">
        <AlertTitle>{$messages.error.controllerError}</AlertTitle>
        <AlertDescription>{data.error}</AlertDescription>
      </Alert>
    {/if}

    <section class="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
		<Card>
        <CardHeader class="flex flex-row items-start justify-between gap-4">
          <div class="space-y-1">
            <CardTitle class="section-title">
              <a class="hover:text-foreground/80 transition-colors" href="/services">{$messages.dashboard.services}</a>
            </CardTitle>
          </div>
          <Badge variant="outline">{data.dashboard?.services.length ?? 0}</Badge>
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
          <CardHeader class="flex flex-row items-start justify-between gap-4">
            <div class="space-y-1">
              <CardTitle class="section-title">
                <a class="hover:text-foreground/80 transition-colors" href="/nodes">{$messages.dashboard.nodes}</a>
              </CardTitle>
            </div>
            <Badge variant="outline">{data.dashboard?.nodes.length ?? 0}</Badge>
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
          <CardHeader class="flex flex-row items-start justify-between gap-4">
            <div class="space-y-1">
              <CardTitle class="section-title">
                <a class="hover:text-foreground/80 transition-colors" href="/tasks">{$messages.dashboard.tasks}</a>
              </CardTitle>
            </div>
            <Badge variant="outline">{totalTaskCount()}</Badge>
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
  </div>
</div>
