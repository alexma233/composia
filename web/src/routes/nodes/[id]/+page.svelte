<script lang="ts">
  import type { PageData, ActionData } from './$types';
  import { enhance } from '$app/forms';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { formatTimestamp, onlineStatusTone, taskStatusTone } from '$lib/presenters';

  export let data: PageData;
  export let form: ActionData;

  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }
</script>

<div class="page-shell">
  <div class="page-stack">
    <Card class="border-border/70 bg-card/95">
      <CardHeader class="gap-4">
        {#if data.node}
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div class="space-y-1">
              <CardTitle class="page-title">{data.node.displayName}</CardTitle>
              <p class="text-sm text-muted-foreground">
                {data.node.nodeId} · last heartbeat {formatTimestamp(data.node.lastHeartbeat)}
              </p>
            </div>
            <Badge variant={onlineStatusTone(data.node.isOnline)}>
              {data.node.isOnline ? 'online' : 'offline'}
            </Badge>
          </div>
        {/if}

        {#if data.error}
          <Alert variant="destructive">
            <AlertTitle>Load failed</AlertTitle>
            <AlertDescription>{data.error}</AlertDescription>
          </Alert>
        {/if}
      </CardHeader>
    </Card>

    <Card class="border-border/70 bg-card/95">
      <CardHeader>
        <CardTitle class="section-title">Docker</CardTitle>
      </CardHeader>
      <CardContent>
        {#if data.dockerStats}
          <div class="space-y-4">
            <div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
              <a href="/nodes/{data.node?.nodeId}/docker/containers" class="rounded-lg border border-border/70 bg-background/80 p-3 transition-colors hover:bg-accent/60">
                <div class="text-2xl font-semibold">{data.dockerStats.containersRunning}/{data.dockerStats.containersTotal}</div>
                <div class="text-xs text-muted-foreground">Containers</div>
              </a>
              <a href="/nodes/{data.node?.nodeId}/docker/images" class="rounded-lg border border-border/70 bg-background/80 p-3 transition-colors hover:bg-accent/60">
                <div class="text-2xl font-semibold">{data.dockerStats.images}</div>
                <div class="text-xs text-muted-foreground">Images</div>
              </a>
              <a href="/nodes/{data.node?.nodeId}/docker/networks" class="rounded-lg border border-border/70 bg-background/80 p-3 transition-colors hover:bg-accent/60">
                <div class="text-2xl font-semibold">{data.dockerStats.networks}</div>
                <div class="text-xs text-muted-foreground">Networks</div>
              </a>
              <a href="/nodes/{data.node?.nodeId}/docker/volumes" class="rounded-lg border border-border/70 bg-background/80 p-3 transition-colors hover:bg-accent/60">
                <div class="text-2xl font-semibold">{data.dockerStats.volumes}</div>
                <div class="text-xs text-muted-foreground">Volumes</div>
              </a>
            </div>

            <div class="text-sm text-muted-foreground">
              Docker {data.dockerStats.dockerServerVersion || 'unknown version'}
              {#if data.dockerStats.volumesSizeBytes > 0}
                · {formatBytes(data.dockerStats.volumesSizeBytes)} in volumes
              {/if}
              {#if data.dockerStats.disksUsageBytes > 0}
                · {formatBytes(data.dockerStats.disksUsageBytes)} disk usage
              {/if}
            </div>

            {#if form?.error}
              <Alert variant="destructive">
                <AlertDescription>{form.error}</AlertDescription>
              </Alert>
            {/if}

            {#if data.node?.isOnline}
              <div class="flex flex-wrap gap-2">
                <form method="POST" action="?/prune" use:enhance>
                  <input type="hidden" name="target" value="all" />
                  <Button variant="outline" size="sm" type="submit">Prune All</Button>
                </form>
                <form method="POST" action="?/prune" use:enhance>
                  <input type="hidden" name="target" value="containers" />
                  <Button variant="outline" size="sm" type="submit">Containers</Button>
                </form>
                <form method="POST" action="?/prune" use:enhance>
                  <input type="hidden" name="target" value="images" />
                  <Button variant="outline" size="sm" type="submit">Images</Button>
                </form>
                <form method="POST" action="?/prune" use:enhance>
                  <input type="hidden" name="target" value="networks" />
                  <Button variant="outline" size="sm" type="submit">Networks</Button>
                </form>
                <form method="POST" action="?/prune" use:enhance>
                  <input type="hidden" name="target" value="volumes" />
                  <Button variant="outline" size="sm" type="submit">Volumes</Button>
                </form>
              </div>
            {:else}
              <div class="text-sm text-muted-foreground">Node is offline. Prune operations require an online node.</div>
            {/if}
          </div>
        {:else}
          <div class="text-sm text-muted-foreground">No Docker stats available. Stats are reported by the agent.</div>
        {/if}
      </CardContent>
    </Card>

    <Card class="border-border/70 bg-card/95">
      <CardHeader>
        <CardTitle class="section-title">Recent tasks</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="space-y-3">
          {#each data.tasks as task}
            <a
              href={`/tasks/${task.taskId}`}
              class="block rounded-lg border border-border/70 bg-background/80 px-4 py-3 transition-colors hover:bg-accent/60"
            >
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div class="min-w-0 flex-1">
                  <div class="truncate text-sm font-medium">{task.type}</div>
                  <div class="truncate text-xs text-muted-foreground">{task.serviceName ?? 'node-level'}</div>
                </div>
                <Badge variant={taskStatusTone(task.status)}>{task.status}</Badge>
              </div>
              <div class="mt-2 text-xs text-muted-foreground">{formatTimestamp(task.createdAt)}</div>
            </a>
          {/each}
          {#if !data.tasks.length}
            <div class="empty-state">No tasks loaded.</div>
          {/if}
        </div>
      </CardContent>
    </Card>
  </div>
</div>
