<script lang="ts">
  import type { PageData } from './$types';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { formatTimestamp, onlineStatusTone, taskStatusTone } from '$lib/presenters';

  export let data: PageData;
</script>

<div class="page-shell">
  <div class="page-stack">
    <Card class="border-border/70 bg-card/95">
      <CardHeader class="gap-4">
        {#if data.node}
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div class="space-y-1">
              <CardTitle class="page-title">{data.node.displayName}</CardTitle>
              <CardDescription class="page-description">
                {data.node.nodeId} · last heartbeat {formatTimestamp(data.node.lastHeartbeat)}
              </CardDescription>
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
      <CardHeader class="space-y-1">
        <CardTitle class="section-title">Recent tasks</CardTitle>
        <CardDescription class="section-description">Latest tasks on this node.</CardDescription>
      </CardHeader>
      <CardContent>
        <div class="space-y-3">
          {#each data.tasks as task}
            <a
              href={`/tasks/${task.taskId}`}
              class="block rounded-lg border border-border/70 bg-background/80 px-4 py-4 transition-colors hover:bg-accent/60"
            >
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <div class="text-sm font-medium">{task.type}</div>
                  <div class="text-xs text-muted-foreground">{task.serviceName || 'system task'}</div>
                </div>
                <Badge variant={taskStatusTone(task.status)}>{task.status}</Badge>
              </div>
              <div class="mt-2 text-sm text-muted-foreground">Created {formatTimestamp(task.createdAt)}</div>
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
