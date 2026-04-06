<script lang="ts">
  import type { PageData } from './$types';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
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

<div class="page-shell">
  <div class="page-stack">
    <section class="grid gap-6 lg:grid-cols-[1.3fr_0.7fr]">
      <Card class="border-border/70 bg-card/95">
        <CardHeader class="gap-4">
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div class="space-y-2">
              <div class="space-y-1">
                <h1 class="page-title">Control plane overview</h1>
                <p class="page-description">Live state for services, nodes, and tasks.</p>
              </div>
            </div>

            <div class="surface-subtle rounded-lg border border-border/70 px-4 py-3 text-right text-sm shadow-xs">
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
        <Card class="border-border/70 bg-card/90">
          <CardHeader class="p-5">
            <CardDescription class="metric-label">Configured nodes</CardDescription>
            <CardTitle class="metric-value">{data.dashboard?.system.configuredNodeCount ?? '-'}</CardTitle>
          </CardHeader>
        </Card>
        <Card class="border-border/70 bg-card/90">
          <CardHeader class="p-5">
            <CardDescription class="metric-label">Online nodes</CardDescription>
            <CardTitle class="metric-value">{data.dashboard?.system.onlineNodeCount ?? '-'}</CardTitle>
          </CardHeader>
        </Card>
        <Card class="border-border/70 bg-card/90">
          <CardHeader class="p-5">
            <CardDescription class="metric-label">Recent tasks</CardDescription>
            <CardTitle class="metric-value">{data.dashboard?.tasks.length ?? 0}</CardTitle>
          </CardHeader>
        </Card>
      </div>
    </section>

    <section class="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
      <Card class="border-border/70 bg-card/95">
        <CardHeader class="flex flex-row items-start justify-between gap-4">
          <div class="space-y-1">
            <CardTitle class="section-title">Services</CardTitle>
            <CardDescription class="section-description">Declared services and runtime state.</CardDescription>
          </div>
          <Badge variant="outline">{data.dashboard?.services.length ?? 0}</Badge>
        </CardHeader>
        <CardContent>
          <div class="space-y-3">
            {#if data.dashboard?.services.length}
              {#each data.dashboard.services as service}
                <a
                  href={`/services/${service.name}`}
                  class="block rounded-lg border border-border/70 bg-background/80 px-4 py-4 transition-colors hover:bg-accent/60"
                >
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
              <div class="empty-state">No service data loaded.</div>
            {/if}
          </div>
        </CardContent>
      </Card>

      <div class="grid gap-6">
        <Card class="border-border/70 bg-card/95">
          <CardHeader class="space-y-1">
            <CardTitle class="section-title">Nodes</CardTitle>
            <CardDescription class="section-description">Heartbeat and availability.</CardDescription>
          </CardHeader>
          <CardContent>
            <div class="space-y-3">
              {#if data.dashboard?.nodes.length}
                {#each data.dashboard.nodes as node}
                  <a
                    href={`/nodes/${node.nodeId}`}
                    class="block rounded-lg border border-border/70 bg-background/80 px-4 py-4 transition-colors hover:bg-accent/60"
                  >
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

        <Card class="border-border/70 bg-card/95">
          <CardHeader class="space-y-1">
            <CardTitle class="section-title">Recent tasks</CardTitle>
            <CardDescription class="section-description">Latest queue activity.</CardDescription>
          </CardHeader>
          <CardContent>
            <div class="space-y-3">
              {#if data.dashboard?.tasks.length}
                {#each data.dashboard.tasks as task}
                  <a
                    href={`/tasks/${task.taskId}`}
                    class="block rounded-lg border border-border/70 bg-background/80 px-4 py-4 transition-colors hover:bg-accent/60"
                  >
                    <div class="flex flex-wrap items-center justify-between gap-3">
                      <div class="min-w-0">
                        <div class="truncate text-sm font-medium">
                          {task.type} {task.serviceName ? `for ${task.serviceName}` : `on ${task.nodeId || 'n/a'}`}
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
                <div class="empty-state">No task data loaded.</div>
              {/if}
            </div>
          </CardContent>
        </Card>
      </div>
    </section>
  </div>
</div>
