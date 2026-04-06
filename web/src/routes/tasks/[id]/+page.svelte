<script lang="ts">
  import { onMount } from 'svelte';

  import type { PageData } from './$types';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
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

<div class="page-shell">
  <div class="page-stack">
    <Card class="border-border/70 bg-card/95">
      <CardHeader class="gap-4">
        {#if data.task}
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div class="space-y-1">
              <CardTitle class="page-title">{data.task.type}</CardTitle>
              <CardDescription class="page-description">
                {data.task.taskId} · {data.task.serviceName || `node: ${data.task.nodeId || 'n/a'}`}
              </CardDescription>
            </div>
            <Badge variant={taskStatusTone(data.task.status)}>
              {data.task.status}
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

      {#if data.task}
        <CardContent class="space-y-4">
          <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <div class="surface-subtle rounded-lg border border-border/70 p-4">
              <div class="metric-label">Source</div>
              <div class="mt-2 text-sm text-foreground">{data.task.source || 'N/A'}</div>
            </div>
            <div class="surface-subtle rounded-lg border border-border/70 p-4">
              <div class="metric-label">Triggered by</div>
              <div class="mt-2 text-sm text-foreground">{data.task.triggeredBy || 'N/A'}</div>
            </div>
            <div class="surface-subtle rounded-lg border border-border/70 p-4">
              <div class="metric-label">Created</div>
              <div class="mt-2 text-sm text-foreground">{formatTimestamp(data.task.createdAt)}</div>
            </div>
            <div class="surface-subtle rounded-lg border border-border/70 p-4">
              <div class="metric-label">Finished</div>
              <div class="mt-2 text-sm text-foreground">{formatTimestamp(data.task.finishedAt)}</div>
            </div>
          </div>

          <div class="grid gap-4 xl:grid-cols-2">
            <div class="rounded-lg border border-border/70 bg-background/80 p-4">
              <div class="metric-label">Repo revision</div>
              <div class="mt-2 break-all text-sm text-foreground">{data.task.repoRevision || 'N/A'}</div>
            </div>
            <div class="rounded-lg border border-border/70 bg-background/80 p-4">
              <div class="metric-label">Result revision</div>
              <div class="mt-2 break-all text-sm text-foreground">{data.task.resultRevision || 'N/A'}</div>
            </div>
          </div>

          <div class="rounded-lg border border-border/70 bg-background/80 p-4">
            <div class="metric-label">Log path</div>
            <div class="mt-2 break-all text-sm text-foreground">{data.task.logPath || 'N/A'}</div>
          </div>

          {#if data.task.errorSummary}
            <Alert variant="destructive">
              <AlertTitle>Task error</AlertTitle>
              <AlertDescription>{data.task.errorSummary}</AlertDescription>
            </Alert>
          {/if}
        </CardContent>
      {/if}
    </Card>

    <Card class="border-border/70 bg-card/95">
      <CardHeader class="space-y-1">
        <CardTitle class="section-title">Task steps</CardTitle>
        <CardDescription class="section-description">Recorded execution steps.</CardDescription>
      </CardHeader>
      <CardContent>
        <div class="space-y-3">
          {#each data.task?.steps ?? [] as step}
            <div class="rounded-lg border border-border/70 bg-background/80 px-4 py-4">
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
            <div class="empty-state">No task steps loaded.</div>
          {/if}
        </div>
      </CardContent>
    </Card>

    <Card class="border-border/70 bg-card/95">
      <CardHeader class="flex flex-row items-center justify-between gap-3">
        <div class="space-y-1">
          <CardTitle class="section-title">Task logs</CardTitle>
          <CardDescription class="section-description">Live log tail for this task.</CardDescription>
        </div>
        <div class="metric-label">{logState}</div>
      </CardHeader>
      <CardContent class="space-y-4">
        {#if logError}
          <Alert variant="destructive">
            <AlertTitle>Log stream failed</AlertTitle>
            <AlertDescription>{logError}</AlertDescription>
          </Alert>
        {/if}

        {#if data.task?.logPath}
          <pre class="max-h-[28rem] overflow-auto rounded-lg border border-border/70 bg-background/80 p-4 font-mono text-xs leading-6 whitespace-pre-wrap break-words">{logContent || 'Waiting for task log output...'}</pre>
        {:else}
          <div class="empty-state">This task does not have a log file.</div>
        {/if}
      </CardContent>
    </Card>
  </div>
</div>
