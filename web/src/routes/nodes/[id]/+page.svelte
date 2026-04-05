<script lang="ts">
  import type { PageData } from './$types';

  import { Badge } from '$lib/components/ui/badge';
  import { formatTimestamp, onlineStatusTone, taskStatusTone } from '$lib/presenters';

  export let data: PageData;
</script>

<div class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="space-y-6">
    <section class="rounded-lg border bg-card p-6 shadow-xs">
      {#if data.node}
        <div class="flex flex-wrap items-center justify-between gap-4">
          <div>
            <div class="text-sm text-muted-foreground">Node detail</div>
            <h1 class="mt-1 text-3xl font-semibold tracking-tight">{data.node.displayName}</h1>
            <div class="mt-2 text-sm text-muted-foreground">
              {data.node.nodeId} · last heartbeat {formatTimestamp(data.node.lastHeartbeat)}
            </div>
          </div>
          <Badge variant={onlineStatusTone(data.node.isOnline)}>
            {data.node.isOnline ? 'online' : 'offline'}
          </Badge>
        </div>
      {/if}

      {#if data.error}
        <div class="mt-4 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
          {data.error}
        </div>
      {/if}
    </section>

    <section class="rounded-lg border bg-card p-6 shadow-xs">
      <h2 class="mb-4 text-xl font-medium">Recent node tasks</h2>
      <div class="space-y-3">
        {#each data.tasks as task}
          <a href={`/tasks/${task.taskId}`} class="block rounded-lg border bg-background px-4 py-4 transition-colors hover:bg-muted/40">
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
          <div class="rounded-lg border border-dashed bg-muted/20 px-4 py-8 text-sm text-muted-foreground">No tasks loaded.</div>
        {/if}
      </div>
    </section>
  </div>
</div>
