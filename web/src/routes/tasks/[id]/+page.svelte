<script lang="ts">
  import { RotateCcw } from 'lucide-svelte';
  import { toast } from 'svelte-sonner';
  import { goto } from '$app/navigation';

  import type { PageData } from './$types';
  import { messages } from '$lib/i18n';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { formatTimestamp, taskStatusLabel, taskStatusTone } from '$lib/presenters';

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  let logContent = $state('');
  let logState = $state('idle');
  let logError = $state('');
  let rerunning = $state(false);

  function isTerminalStatus(status: string): boolean {
    return status === 'succeeded' || status === 'failed' || status === 'cancelled';
  }

  async function runAgain() {
    if (!data.task?.taskId || rerunning) {
      return;
    }

    rerunning = true;

    try {
      const response = await fetch(`/tasks/${data.task.taskId}/rerun`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });

      const payload = await response.json();
      if (!response.ok || !payload.taskId) {
        throw new Error(payload.error ?? 'Failed to run task again.');
      }

      toast.success(`Task rerun started: ${payload.taskId.slice(0, 12)}`);
      goto(`/tasks/${payload.taskId}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to run task again.');
    } finally {
      rerunning = false;
    }
  }

  $effect(() => {
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
		<Card>
			<CardHeader>
        {#if data.task}
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div class="space-y-1">
              <CardTitle class="page-title">{data.task.type}</CardTitle>
              <CardDescription class="page-description">
                {data.task.taskId} · {data.task.serviceName || `${$messages.tasks.nodeLevel}: ${data.task.nodeId || 'n/a'}`}
              </CardDescription>
            </div>
            <div class="flex items-center gap-2">
              <Badge variant={taskStatusTone(data.task.status)}>
                {taskStatusLabel(data.task.status, $messages)}
              </Badge>
              {#if isTerminalStatus(data.task.status)}
                <Button type="button" variant="outline" size="sm" onclick={runAgain} disabled={rerunning}>
                  <RotateCcw class="mr-2 size-4" />
                  {rerunning ? $messages.tasks.running : $messages.tasks.runAgain}
                </Button>
              {/if}
            </div>
          </div>
        {/if}

        {#if data.error}
          <Alert variant="destructive">
            <AlertTitle>{$messages.error.loadFailed}</AlertTitle>
            <AlertDescription>{data.error}</AlertDescription>
          </Alert>
        {/if}
      </CardHeader>

      {#if data.task}
        <CardContent class="space-y-4">
          <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <div class="metric-card">
              <div class="metric-label">{$messages.tasks.taskDetails.source}</div>
              <div class="mt-2 text-sm text-foreground">{data.task.source || $messages.common.na}</div>
            </div>
            <div class="metric-card">
              <div class="metric-label">{$messages.tasks.taskDetails.triggeredBy}</div>
              <div class="mt-2 text-sm text-foreground">{data.task.triggeredBy || $messages.common.na}</div>
            </div>
            <div class="metric-card">
              <div class="metric-label">{$messages.tasks.taskDetails.created}</div>
              <div class="mt-2 text-sm text-foreground">{formatTimestamp(data.task.createdAt)}</div>
            </div>
            <div class="metric-card">
              <div class="metric-label">{$messages.tasks.taskDetails.finished}</div>
              <div class="mt-2 text-sm text-foreground">{formatTimestamp(data.task.finishedAt)}</div>
            </div>
          </div>

          <div class="grid gap-4 xl:grid-cols-2">
            <div class="inset-card">
              <div class="metric-label">{$messages.tasks.taskDetails.repoRevision}</div>
              <div class="mt-2 break-all text-sm text-foreground">{data.task.repoRevision || $messages.common.na}</div>
            </div>
            <div class="inset-card">
              <div class="metric-label">{$messages.tasks.taskDetails.resultRevision}</div>
              <div class="mt-2 break-all text-sm text-foreground">{data.task.resultRevision || $messages.common.na}</div>
            </div>
          </div>

          <div class="inset-card">
            <div class="metric-label">{$messages.tasks.taskDetails.logPath}</div>
            <div class="mt-2 break-all text-sm text-foreground">{data.task.logPath || $messages.common.na}</div>
          </div>

          {#if data.task.errorSummary}
            <Alert variant="destructive">
              <AlertTitle>{$messages.error.taskError}</AlertTitle>
              <AlertDescription>{data.task.errorSummary}</AlertDescription>
            </Alert>
          {/if}
        </CardContent>
      {/if}
    </Card>

		<Card>
      <CardHeader class="space-y-1">
        <CardTitle class="section-title">{$messages.tasks.taskSteps}</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="space-y-3">
          {#each data.task?.steps ?? [] as step}
            <div class="inset-card px-4 py-4">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div class="text-sm font-medium">{step.stepName}</div>
                <Badge variant={taskStatusTone(step.status)}>{taskStatusLabel(step.status, $messages)}</Badge>
              </div>
              <div class="mt-2 text-sm text-muted-foreground">
                {formatTimestamp(step.startedAt)} to {formatTimestamp(step.finishedAt)}
              </div>
            </div>
          {/each}
          {#if !(data.task?.steps?.length ?? 0)}
            <div class="empty-state">{$messages.common.noData}</div>
          {/if}
        </div>
      </CardContent>
    </Card>

		<Card>
      <CardHeader class="flex flex-row items-center justify-between gap-3">
        <CardTitle class="section-title">{$messages.tasks.taskLogs}</CardTitle>
        <div class="metric-label">{logState}</div>
      </CardHeader>
      <CardContent class="space-y-4">
        {#if logError}
          <Alert variant="destructive">
            <AlertTitle>{$messages.error.logStreamFailed}</AlertTitle>
            <AlertDescription>{logError}</AlertDescription>
          </Alert>
        {/if}

        {#if data.task?.logPath}
          <pre class="code-surface max-h-[28rem] overflow-auto">{logContent || $messages.tasks.waitingForOutput}</pre>
        {:else}
          <div class="empty-state">{$messages.tasks.noLogFile}</div>
        {/if}
      </CardContent>
    </Card>
  </div>
</div>
