<script lang="ts">
  import { onMount } from 'svelte';

  import type { PageData } from './$types';

  import { Badge } from '$lib/components/ui/badge';
  import { formatTimestamp, taskStatusTone } from '$lib/presenters';

  export let data: PageData;

  let logContent = '';
  let logState = 'idle';
  let logError = '';

  onMount(() => {
    if (!data.task?.taskId || !data.task.logPath) {
      return;
    }

    const controller = new AbortController();
    const decoder = new TextDecoder();
    logState = 'connecting';

    void (async () => {
      try {
        const response = await fetch(`/tasks/${data.task?.taskId}/logs`, {
          signal: controller.signal
        });
        if (!response.ok || !response.body) {
          throw new Error(`Failed to tail task logs: ${response.status}`);
        }

        logState = 'streaming';
        const reader = response.body.getReader();

        while (true) {
          const { done, value } = await reader.read();
          if (done) {
            break;
          }
          if (value) {
            logContent += decoder.decode(value, { stream: true });
          }
        }
      } catch (error) {
        if (controller.signal.aborted) {
          return;
        }
        logState = 'failed';
        logError = error instanceof Error ? error.message : 'Failed to tail task logs.';
      }
    })();

    return () => controller.abort();
  });
</script>

<div class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
  <div class="space-y-6">
    <section class="rounded-lg border bg-card p-6 shadow-xs">
      {#if data.task}
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div>
            <div class="text-sm text-muted-foreground">Task detail</div>
            <h1 class="mt-1 text-3xl font-semibold tracking-tight">{data.task.type}</h1>
            <div class="mt-2 text-sm text-muted-foreground">
              {data.task.taskId} · {data.task.serviceName || 'system task'} · {data.task.nodeId || 'n/a'}
            </div>
          </div>
          <Badge variant={taskStatusTone(data.task.status)}>
            {data.task.status}
          </Badge>
        </div>

        <div class="mt-6 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <div class="rounded-lg border bg-background p-4">
            <div class="text-xs uppercase tracking-[0.2em] text-muted-foreground">Source</div>
            <div class="mt-2 text-sm">{data.task.source || 'N/A'}</div>
          </div>
          <div class="rounded-lg border bg-background p-4">
            <div class="text-xs uppercase tracking-[0.2em] text-muted-foreground">Triggered by</div>
            <div class="mt-2 text-sm">{data.task.triggeredBy || 'N/A'}</div>
          </div>
          <div class="rounded-lg border bg-background p-4">
            <div class="text-xs uppercase tracking-[0.2em] text-muted-foreground">Created</div>
            <div class="mt-2 text-sm">{formatTimestamp(data.task.createdAt)}</div>
          </div>
          <div class="rounded-lg border bg-background p-4">
            <div class="text-xs uppercase tracking-[0.2em] text-muted-foreground">Finished</div>
            <div class="mt-2 text-sm">{formatTimestamp(data.task.finishedAt)}</div>
          </div>
        </div>

        <div class="mt-6 grid gap-4 xl:grid-cols-2">
          <div class="rounded-lg border bg-background p-4">
            <div class="text-xs uppercase tracking-[0.2em] text-muted-foreground">Repo revision</div>
            <div class="mt-2 break-all text-sm">{data.task.repoRevision || 'N/A'}</div>
          </div>
          <div class="rounded-lg border bg-background p-4">
            <div class="text-xs uppercase tracking-[0.2em] text-muted-foreground">Result revision</div>
            <div class="mt-2 break-all text-sm">{data.task.resultRevision || 'N/A'}</div>
          </div>
        </div>

        <div class="mt-4 rounded-lg border bg-background p-4">
          <div class="text-xs uppercase tracking-[0.2em] text-muted-foreground">Log path</div>
          <div class="mt-2 break-all text-sm">{data.task.logPath || 'N/A'}</div>
        </div>

        {#if data.task.errorSummary}
          <div class="mt-4 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
            {data.task.errorSummary}
          </div>
        {/if}
      {/if}

      {#if data.error}
        <div class="mt-4 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
          {data.error}
        </div>
      {/if}
    </section>

    <section class="rounded-lg border bg-card p-6 shadow-xs">
      <h2 class="mb-4 text-xl font-medium">Task steps</h2>
      <div class="space-y-3">
        {#each data.task?.steps ?? [] as step}
          <div class="rounded-lg border bg-background px-4 py-4">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div class="text-sm font-medium">{step.stepName}</div>
              <Badge variant={taskStatusTone(step.status)}>{step.status}</Badge>
            </div>
            <div class="mt-2 text-sm text-muted-foreground">
              {formatTimestamp(step.startedAt)} to {formatTimestamp(step.finishedAt)}
            </div>
          </div>
        {/each}
        {#if !(data.task?.steps?.length ?? 0)}
          <div class="rounded-lg border border-dashed bg-muted/20 px-4 py-8 text-sm text-muted-foreground">No task steps loaded.</div>
        {/if}
      </div>
    </section>

    <section class="rounded-lg border bg-card p-6 shadow-xs">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 class="text-xl font-medium">Task logs</h2>
        <div class="text-xs uppercase tracking-[0.2em] text-muted-foreground">{logState}</div>
      </div>

      {#if logError}
        <div class="mb-4 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
          {logError}
        </div>
      {/if}

      {#if data.task?.logPath}
        <pre class="max-h-[28rem] overflow-auto rounded-lg border bg-background p-4 font-mono text-xs leading-6 whitespace-pre-wrap break-words">{logContent || 'Waiting for task log output...'}</pre>
      {:else}
        <div class="rounded-lg border border-dashed bg-muted/20 px-4 py-8 text-sm text-muted-foreground">
          This task does not have a log file.
        </div>
      {/if}
    </section>
  </div>
</div>
