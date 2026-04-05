<script lang="ts">
  import type { PageData } from './$types';

  import { Badge } from '$lib/components/ui/badge';
  import { formatTimestamp, runtimeStatusTone, taskStatusTone } from '$lib/presenters';

  export let data: PageData;
</script>

<div class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="space-y-6">
    <section class="rounded-lg border bg-card p-6 shadow-xs">
      {#if data.service}
        <div class="flex flex-wrap items-center justify-between gap-4">
          <div>
            <div class="text-sm text-muted-foreground">Service detail</div>
            <h1 class="mt-1 text-3xl font-semibold tracking-tight">{data.service.name}</h1>
            <div class="mt-2 text-sm text-muted-foreground">
              Node {data.service.node} · updated {formatTimestamp(data.service.updatedAt)}
            </div>
          </div>
          <div class="flex items-center gap-3">
            <a href={`/services/${data.service.name}/secret`} class="inline-flex h-9 items-center rounded-md border bg-background px-4 text-sm transition-colors hover:bg-muted/40">Edit secret</a>
            <Badge variant={runtimeStatusTone(data.service.runtimeStatus)}>
              {data.service.runtimeStatus}
            </Badge>
          </div>
        </div>
      {:else}
        <h1 class="text-3xl font-semibold tracking-tight">Service detail</h1>
      {/if}

      {#if data.error}
        <div class="mt-4 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
          {data.error}
        </div>
      {/if}
    </section>

    <section class="grid gap-6 xl:grid-cols-2">
      <article class="rounded-lg border bg-card p-6 shadow-xs">
        <h2 class="mb-4 text-xl font-medium">Recent tasks</h2>
        <div class="space-y-3">
          {#each data.tasks as task}
            <a href={`/tasks/${task.taskId}`} class="block rounded-lg border bg-background px-4 py-4 transition-colors hover:bg-muted/40">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <div class="text-sm font-medium">{task.type}</div>
                  <div class="text-xs text-muted-foreground">{task.taskId}</div>
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
      </article>

      <article class="rounded-lg border bg-card p-6 shadow-xs">
        <h2 class="mb-4 text-xl font-medium">Recent backups</h2>
        <div class="space-y-3">
          {#each data.backups as backup}
            <div class="rounded-lg border bg-background px-4 py-4">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <div class="text-sm font-medium">{backup.dataName}</div>
                  <div class="text-xs text-muted-foreground">{backup.backupId}</div>
                </div>
                <Badge variant={taskStatusTone(backup.status)}>{backup.status}</Badge>
              </div>
              <div class="mt-2 text-sm text-muted-foreground">Finished {formatTimestamp(backup.finishedAt || backup.startedAt)}</div>
            </div>
          {/each}
          {#if !data.backups.length}
            <div class="rounded-lg border border-dashed bg-muted/20 px-4 py-8 text-sm text-muted-foreground">No backups loaded.</div>
          {/if}
        </div>
      </article>
    </section>
  </div>
</div>
