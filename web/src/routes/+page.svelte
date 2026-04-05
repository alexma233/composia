<script lang="ts">
  import type { PageData } from './$types';

  import { Badge } from '$lib/components/ui/badge';
  import {
    formatTimestamp,
    onlineStatusTone,
    runtimeStatusTone,
    taskStatusTone
  } from '$lib/presenters';

  export let data: PageData;

  function onlineSummary() {
    if (!data.dashboard) return 'No runtime data';
    return `${data.dashboard.system.onlineNodeCount}/${data.dashboard.system.configuredNodeCount} nodes online`;
  }
</script>

<svelte:head>
  <title>Composia Control Plane</title>
  <meta
    name="description"
    content="Composia controller overview with live services, nodes, and task history."
  />
</svelte:head>

<div class="mx-auto min-h-screen max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="space-y-8">
    <section class="grid gap-6 lg:grid-cols-[1.3fr_0.7fr]">
      <div class="rounded-lg border bg-card p-6 shadow-xs">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="space-y-3">
            <p class="text-sm uppercase tracking-[0.28em] text-primary">Composia</p>
            <div class="space-y-2">
              <h1 class="text-3xl font-semibold tracking-tight md:text-4xl">
                Control plane overview
              </h1>
              <p class="max-w-3xl text-sm leading-6 text-muted-foreground md:text-base">
                Live controller state for services, nodes, and recent task activity.
              </p>
            </div>
          </div>

          <div class="rounded-lg border bg-muted/50 px-4 py-3 text-right text-sm">
            <div class="font-medium">{data.dashboard?.system.version ?? 'Controller unavailable'}</div>
            <div class="text-muted-foreground">{onlineSummary()}</div>
          </div>
        </div>

        {#if data.error}
          <div class="mt-6 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
            {data.error}
          </div>
        {/if}
      </div>

      <div class="grid gap-4 sm:grid-cols-3 lg:grid-cols-1">
        <article class="rounded-lg border bg-card p-5 shadow-xs">
          <div class="text-sm text-muted-foreground">Configured nodes</div>
          <div class="mt-2 text-3xl font-semibold">
            {data.dashboard?.system.configuredNodeCount ?? '-'}
          </div>
        </article>
        <article class="rounded-lg border bg-card p-5 shadow-xs">
          <div class="text-sm text-muted-foreground">Online nodes</div>
          <div class="mt-2 text-3xl font-semibold">
            {data.dashboard?.system.onlineNodeCount ?? '-'}
          </div>
        </article>
        <article class="rounded-lg border bg-card p-5 shadow-xs">
          <div class="text-sm text-muted-foreground">Recent tasks shown</div>
          <div class="mt-2 text-3xl font-semibold">
            {data.dashboard?.tasks.length ?? 0}
          </div>
        </article>
      </div>
    </section>

    <section class="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
      <article class="rounded-lg border bg-card p-6 shadow-xs">
        <div class="mb-5 flex items-center justify-between gap-4">
          <div>
            <h2 class="text-xl font-medium">Services</h2>
            <p class="text-sm text-muted-foreground">Current declared services and runtime state</p>
          </div>
          <span class="rounded-md border bg-muted/50 px-2.5 py-1 text-xs text-muted-foreground">
            {data.dashboard?.services.length ?? 0} loaded
          </span>
        </div>

        <div class="space-y-3">
          {#if data.dashboard?.services.length}
            {#each data.dashboard.services as service}
              <a href={`/services/${service.name}`} class="block rounded-lg border bg-background px-4 py-4 transition-colors hover:bg-muted/40">
                <div class="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <div class="text-base font-medium">{service.name}</div>
                    <div class="text-sm text-muted-foreground">
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
            <div class="rounded-lg border border-dashed bg-muted/20 px-4 py-8 text-sm text-muted-foreground">
              No service data loaded.
            </div>
          {/if}
        </div>
      </article>

      <div class="grid gap-6">
        <article class="rounded-lg border bg-card p-6 shadow-xs">
          <div class="mb-5">
            <h2 class="text-xl font-medium">Nodes</h2>
            <p class="text-sm text-muted-foreground">Configured nodes and heartbeat state</p>
          </div>

          <div class="space-y-3">
            {#if data.dashboard?.nodes.length}
            {#each data.dashboard.nodes as node}
                <a href={`/nodes/${node.nodeId}`} class="block rounded-lg border bg-background px-4 py-4 transition-colors hover:bg-muted/40">
                  <div class="flex flex-wrap items-center justify-between gap-3">
                    <div>
                      <div class="text-base font-medium">{node.displayName}</div>
                      <div class="text-sm text-muted-foreground">{node.nodeId}</div>
                    </div>
                    <Badge variant={onlineStatusTone(node.isOnline)}>
                      {node.isOnline ? 'online' : 'offline'}
                    </Badge>
                  </div>
                  <div class="mt-3 text-sm text-muted-foreground">
                    Last heartbeat: {formatTimestamp(node.lastHeartbeat)}
                  </div>
                </a>
              {/each}
            {:else}
              <div class="rounded-lg border border-dashed bg-muted/20 px-4 py-8 text-sm text-muted-foreground">
                No node data loaded.
              </div>
            {/if}
          </div>
        </article>

        <article class="rounded-lg border bg-card p-6 shadow-xs">
          <div class="mb-5">
            <h2 class="text-xl font-medium">Recent tasks</h2>
            <p class="text-sm text-muted-foreground">Latest task activity from the durable queue</p>
          </div>

          <div class="space-y-3">
            {#if data.dashboard?.tasks.length}
            {#each data.dashboard.tasks as task}
                <a href={`/tasks/${task.taskId}`} class="block rounded-lg border bg-background px-4 py-4 transition-colors hover:bg-muted/40">
                  <div class="flex flex-wrap items-center justify-between gap-3">
                    <div class="min-w-0">
                      <div class="truncate text-sm font-medium">
                        {task.type} {task.serviceName ? `for ${task.serviceName}` : ''}
                      </div>
                      <div class="text-xs text-muted-foreground">
                        {task.taskId} on {task.nodeId || 'n/a'}
                      </div>
                    </div>
                    <Badge variant={taskStatusTone(task.status)}>
                      {task.status}
                    </Badge>
                  </div>
                  <div class="mt-3 text-sm text-muted-foreground">
                    Created {formatTimestamp(task.createdAt)}
                  </div>
                </a>
              {/each}
            {:else}
              <div class="rounded-lg border border-dashed bg-muted/20 px-4 py-8 text-sm text-muted-foreground">
                No task data loaded.
              </div>
            {/if}
          </div>
        </article>
      </div>
    </section>
  </div>
</div>
