<script lang="ts">
  import type { PageData } from './$types';
  import type { Snippet } from 'svelte';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { formatTimestamp, onlineStatusTone, runtimeStatusTone } from '$lib/presenters';
  import TaskItem from '$lib/components/app/task-item.svelte';

  interface Props {
    data: PageData;
    children?: Snippet;
  }

  let { data }: Props = $props();

  function onlineSummary() {
    if (!data.dashboard) return 'No runtime data';
    return `${data.dashboard.system.onlineNodeCount}/${data.dashboard.system.configuredNodeCount} nodes online`;
  }

  function isTaskRecent(createdAt: string) {
    const createdAtMs = Date.parse(createdAt);
    if (Number.isNaN(createdAtMs)) return false;
    return Date.now() - createdAtMs <= 24 * 60 * 60 * 1000;
  }

  let recentTasks = $derived((data.dashboard?.tasks ?? [])
    .filter((t) => isTaskRecent(t.createdAt))
    .slice(0, 6));
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
    <section class="grid gap-6 lg:grid-cols-[1.3fr_0.7fr]">
		<Card>
			<CardHeader>
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div class="space-y-2">
              <div class="space-y-1">
                <h1 class="page-title">Control plane overview</h1>
                <p class="page-description">Live state for services, nodes, and tasks.</p>
              </div>
            </div>

            <div class="metric-card px-4 py-3 text-right text-sm">
              <div class="font-medium text-foreground">
                {data.dashboard?.system.version ?? 'Controller unavailable'}
              </div>
              <div class="text-muted-foreground">{onlineSummary()}</div>
            </div>
          </div>

          {#if data.error}
            <Alert variant="destructive">
              <AlertTitle>Controller error</AlertTitle>
              <AlertDescription>{data.error}</AlertDescription>
            </Alert>
          {/if}
        </CardHeader>
      </Card>

		<div class="grid gap-4 sm:grid-cols-3 lg:grid-cols-1">
			<Card>
				<CardHeader class="p-5">
            <CardDescription class="metric-label">Configured nodes</CardDescription>
            <CardTitle class="metric-value">{data.dashboard?.system.configuredNodeCount ?? '-'}</CardTitle>
          </CardHeader>
        </Card>
			<Card>
				<CardHeader class="p-5">
            <CardDescription class="metric-label">Online nodes</CardDescription>
            <CardTitle class="metric-value">{data.dashboard?.system.onlineNodeCount ?? '-'}</CardTitle>
          </CardHeader>
        </Card>
			<Card>
				<CardHeader class="p-5">
            <CardDescription class="metric-label">Recent tasks</CardDescription>
            <CardTitle class="metric-value">{recentTasks.length}</CardTitle>
          </CardHeader>
        </Card>
      </div>
    </section>

    <section class="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
		<Card>
        <CardHeader class="flex flex-row items-start justify-between gap-4">
          <div class="space-y-1">
            <CardTitle class="section-title">Services</CardTitle>
          </div>
          <Badge variant="outline">{data.dashboard?.services.length ?? 0}</Badge>
        </CardHeader>
        <CardContent>
          <div class="space-y-3">
            {#if data.dashboard?.services.length}
              {#each data.dashboard.services as service}
                <a
                  href={`/services/${service.name}`}
                  class="list-row"
                >
                  <div class="flex flex-wrap items-center justify-between gap-3">
                    <div class="min-w-0 flex-1">
                      <div class="truncate text-sm font-medium">{service.name}</div>
                      <div class="truncate text-xs text-muted-foreground">
                        Updated {formatTimestamp(service.updatedAt)}
                      </div>
                    </div>
                    <Badge variant={runtimeStatusTone(service.runtimeStatus)}>
                      {service.runtimeStatus}
                    </Badge>
                  </div>
                </a>
              {/each}
            {:else}
              <div class="empty-state">No service data loaded.</div>
            {/if}
          </div>
        </CardContent>
      </Card>

      <div class="grid gap-6">
			<Card>
          <CardHeader class="flex flex-row items-start justify-between gap-4">
            <div class="space-y-1">
              <CardTitle class="section-title">Nodes</CardTitle>
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
                        {node.isOnline ? 'online' : 'offline'}
                      </Badge>
                    </div>
                    <div class="mt-2 text-xs text-muted-foreground">
                      Last heartbeat {formatTimestamp(node.lastHeartbeat)}
                    </div>
                  </a>
                {/each}
              {:else}
                <div class="empty-state">No node data loaded.</div>
              {/if}
            </div>
          </CardContent>
        </Card>

			<Card>
          <CardHeader class="flex flex-row items-start justify-between gap-4">
            <div class="space-y-1">
              <CardTitle class="section-title">Recent tasks</CardTitle>
            </div>
            <Badge variant="outline">{recentTasks.length}</Badge>
          </CardHeader>
          <CardContent>
            <div class="space-y-3">
              {#if recentTasks.length}
                {#each recentTasks as task}
                  <TaskItem {task} showService />
                {/each}
              {:else}
                <div class="empty-state">No recent tasks in the last 24 hours.</div>
              {/if}
            </div>
          </CardContent>
        </Card>
      </div>
    </section>
  </div>
</div>
