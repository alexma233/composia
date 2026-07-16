<script lang="ts">
  import { onDestroy } from 'svelte';
  import { RotateCcw } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { goto, invalidate } from '$app/navigation';

  import type { PageData } from './$types';
  import { getMessages } from '$lib/i18n';
  import { logLiveMode } from '$lib/log-accessibility';

  const messages = getMessages();
  import { actionErrorMessage } from '$lib/capabilities';

  import TerminalSurface from '$lib/components/app/terminal-surface.svelte';
  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import * as Dialog from '$lib/components/ui/dialog';
  import { Label } from '$lib/components/ui/label';
  import { startPolling } from '$lib/refresh';
  import { formatTimestamp, taskStatusLabel, taskStatusTone, taskStepNameLabel, taskTypeLabel } from '$lib/presenters';
  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  let logContent = $state('');
  let accessibleLogChunks = $state<string[]>([]);
  let logState = $state('idle');
  let logError = $state('');
  let logStreamTaskId = $state('');
  let logStreamController = $state<AbortController | null>(null);
  let rerunning = $state(false);
  let resolvingConfirmation = $state(false);
  let rollbackDialogOpen = $state(false);
  let creatingRollback = $state(false);
  let rollbackDns = $state(true);
  let deploySource = $state(false);
  let stopTarget = $state(false);
  let stopTaskRefreshHandle: null | (() => void) = null;
  let taskRefreshTaskId = '';
  let pendingAccessibleLogChunk = '';
  let logAccessibilityTimer: ReturnType<typeof setTimeout> | null = null;

  function isTerminalStatus(status: string): boolean {
    return status === 'succeeded' || status === 'failed' || status === 'cancelled';
  }

  function logStateLabel(state: string): string {
    switch (state) {
      case 'idle':
        return $messages.tasks.logStreamStatus.idle;
      case 'connecting':
        return $messages.tasks.logStreamStatus.connecting;
      case 'streaming':
        return $messages.tasks.logStreamStatus.streaming;
      case 'completed':
        return $messages.tasks.logStreamStatus.completed;
      case 'failed':
        return $messages.tasks.logStreamStatus.failed;
      default:
        return state;
    }
  }

  function appendLogContent(chunk: string) {
    logContent += chunk;
    queueAccessibleLogUpdate(chunk);
  }

  function queueAccessibleLogUpdate(chunk: string) {
    pendingAccessibleLogChunk += chunk;
    if (logAccessibilityTimer) {
      return;
    }

    logAccessibilityTimer = setTimeout(flushAccessibleLogUpdate, 1000);
  }

  function flushAccessibleLogUpdate() {
    if (logAccessibilityTimer) {
      clearTimeout(logAccessibilityTimer);
      logAccessibilityTimer = null;
    }
    if (pendingAccessibleLogChunk) {
      accessibleLogChunks = [...accessibleLogChunks, pendingAccessibleLogChunk];
      pendingAccessibleLogChunk = '';
    }
  }

  function resetAccessibleLog() {
    if (logAccessibilityTimer) {
      clearTimeout(logAccessibilityTimer);
      logAccessibilityTimer = null;
    }
    pendingAccessibleLogChunk = '';
    accessibleLogChunks = [];
  }

  $effect(() => {
    const taskId = data.task?.taskId ?? '';
    const taskStatus = data.task?.status ?? '';
    if (!taskId || isTerminalStatus(taskStatus)) {
      stopTaskRefresh();
      return;
    }

    if (taskRefreshTaskId === taskId && stopTaskRefreshHandle) {
      return;
    }

    startTaskRefresh(taskId);
  });

  function startTaskRefresh(taskId: string) {
    stopTaskRefresh();
    taskRefreshTaskId = taskId;
    stopTaskRefreshHandle = startPolling(() => invalidate('app:task-detail'), {
      intervalMs: 2500,
      errorIntervalMs: 4000,
      initialDelayMs: 1200,
    });
  }

  function stopTaskRefresh() {
    stopTaskRefreshHandle?.();
    stopTaskRefreshHandle = null;
    taskRefreshTaskId = '';
  }

  function isAwaitingMigrationConfirmation(): boolean {
    return data.task?.type === 'migrate' && data.task?.status === 'awaiting_confirmation';
  }

  function canCreateMigrationRollback(): boolean {
    return (
      data.task?.type === 'migrate' &&
      ['awaiting_confirmation', 'cancelled', 'failed'].includes(data.task.status)
    );
  }

  function hasSucceededStep(stepName: string): boolean {
    return data.task?.steps?.some((step) => step.stepName === stepName && step.status === 'succeeded') ?? false;
  }

  function canRollbackDns(): boolean {
    return hasSucceededStep('dns_update');
  }

  function canDeploySource(): boolean {
    return hasSucceededStep('compose_down');
  }

  function canStopTarget(): boolean {
    return hasSucceededStep('compose_up');
  }

  function openRollbackDialog() {
    rollbackDns = canRollbackDns();
    deploySource = false;
    stopTarget = false;
    rollbackDialogOpen = true;
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
        throw new Error(actionErrorMessage(payload, $messages, $messages.tasks.rerunFailed));
      }

      toast.success($messages.tasks.rerunStarted.replace('{taskId}', payload.taskId.slice(0, 12)));
      goto(`/tasks/${payload.taskId}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : $messages.tasks.rerunFailed);
    } finally {
      rerunning = false;
    }
  }

  async function resolveConfirmation(decision: 'approve' | 'reject') {
    if (!data.task?.taskId || resolvingConfirmation) {
      return;
    }

    resolvingConfirmation = true;

    try {
      const response = await fetch(`/tasks/${data.task.taskId}/confirmation`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ decision }),
      });

      const payload = await response.json();
      if (!response.ok || !payload.taskId) {
        throw new Error(actionErrorMessage(payload, $messages, $messages.tasks.resolveConfirmationFailed));
      }

      if (decision === 'approve') {
        toast.success($messages.tasks.resumed.replace('{taskId}', payload.taskId.slice(0, 12)));
      } else {
        toast.success($messages.tasks.cancelledWithTaskId.replace('{taskId}', payload.taskId.slice(0, 12)));
      }
      goto(`/tasks/${payload.taskId}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : $messages.tasks.resolveConfirmationFailed);
    } finally {
      resolvingConfirmation = false;
    }
  }

  async function createRollback() {
    if (!data.task?.taskId || creatingRollback) {
      return;
    }

    creatingRollback = true;

    try {
      const response = await fetch(`/tasks/${data.task.taskId}/rollback`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          rollbackDns,
          deploySource,
          stopTarget,
          cleanupTarget: false,
        }),
      });

      const payload = await response.json();
      if (!response.ok || !payload.taskId) {
        throw new Error(actionErrorMessage(payload, $messages, $messages.tasks.rollbackFailed));
      }

      toast.success($messages.tasks.rollbackQueued.replace('{taskId}', payload.taskId.slice(0, 12)));
      rollbackDialogOpen = false;
      goto(`/tasks/${payload.taskId}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : $messages.tasks.rollbackFailed);
    } finally {
      creatingRollback = false;
    }
  }

  $effect(() => {
    const nextTaskId = data.task?.taskId ?? '';
    if (nextTaskId && nextTaskId !== logStreamTaskId) {
      void startLogStream(nextTaskId);
    }
  });

  $effect(() => {
    if (data.task?.taskId || !logStreamTaskId) {
      return;
    }
    stopLogStream();
    logStreamTaskId = '';
    logContent = '';
    logError = '';
    logState = 'idle';
    resetAccessibleLog();
  });

  onDestroy(() => {
    stopTaskRefresh();
    stopLogStream();
    resetAccessibleLog();
  });

  async function startLogStream(taskId: string) {
    stopLogStream();
    const controller = new AbortController();
    const decoder = new TextDecoder();
    logStreamController = controller;
    logStreamTaskId = taskId;
    logContent = '';
    logError = '';
    logState = 'connecting';
    resetAccessibleLog();

    try {
      const response = await fetch(`/tasks/${taskId}/logs`, {
        signal: controller.signal
      });
      if (!response.ok || !response.body) {
        throw new Error(`${$messages.error.logStreamFailed}: ${response.status}`);
      }

      logState = 'streaming';
      const reader = response.body.getReader();

      while (true) {
        const { done, value } = await reader.read();
        if (done) {
          break;
        }
        if (value) {
          appendLogContent(decoder.decode(value, { stream: true }));
        }
      }
      const trailing = decoder.decode();
      if (trailing) {
        appendLogContent(trailing);
      }
      flushAccessibleLogUpdate();
      logState = 'completed';
    } catch (error) {
      if (controller.signal.aborted) {
        return;
      }
      flushAccessibleLogUpdate();
      logState = 'failed';
      logError = error instanceof Error ? error.message : $messages.error.logStreamFailed;
    } finally {
      if (logStreamController === controller) {
        logStreamController = null;
      }
    }
  }

  function retryLogStream() {
    const taskId = data.task?.taskId ?? logStreamTaskId;
    if (taskId) {
      void startLogStream(taskId);
    }
  }

  function stopLogStream() {
    logStreamController?.abort();
    logStreamController = null;
  }
</script>

<svelte:head>
  <title>{data.task?.type ? `${taskTypeLabel(data.task.type, $messages)} - ` : ''}{$messages.tasks.pageTitle}</title>
</svelte:head>

<div class="page-shell">
  <div class="page-stack">
		<Card>
			<CardHeader>
        {#if data.task}
          <div class="page-header">
            <div class="page-heading">
              <CardTitle class="page-title" level="1">{taskTypeLabel(data.task.type, $messages)}</CardTitle>
              <div class="page-meta">
                {data.task.taskId} · {data.task.serviceName || `${$messages.tasks.nodeLevel}: ${data.task.nodeId || $messages.common.na}`}
              </div>
            </div>
            <div class="flex items-center gap-2">
              <Badge variant={taskStatusTone(data.task.status)}>
                {taskStatusLabel(data.task.status, $messages)}
              </Badge>
              {#if isTerminalStatus(data.task.status)}
                {#if canCreateMigrationRollback()}
                  <Button type="button" variant="outline" size="sm" onclick={openRollbackDialog}>
                    {$messages.tasks.rollback}
                  </Button>
                {/if}
                <Button type="button" variant="outline" size="sm" onclick={runAgain} disabled={rerunning}>
                  <RotateCcw class="mr-2 size-4" />
                  {rerunning ? $messages.tasks.running : $messages.tasks.runAgain}
                </Button>
              {:else if isAwaitingMigrationConfirmation()}
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onclick={openRollbackDialog}
                  disabled={resolvingConfirmation}
                >
                  {$messages.tasks.rollback}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onclick={() => resolveConfirmation('reject')}
                  disabled={resolvingConfirmation}
                >
                  {resolvingConfirmation ? $messages.tasks.resolving : $messages.tasks.reject}
                </Button>
                <Button
                  type="button"
                  size="sm"
                  onclick={() => resolveConfirmation('approve')}
                  disabled={resolvingConfirmation}
                >
                  {resolvingConfirmation ? $messages.tasks.resolving : $messages.tasks.approve}
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
          <div class="summary-grid">
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
      <CardHeader>
        <CardTitle class="section-title" level="2">{$messages.tasks.taskSteps}</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="space-y-3">
          {#each data.task?.steps ?? [] as step}
            <div class="inset-card">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div class="text-sm font-medium">{taskStepNameLabel(step.stepName, $messages)}</div>
                <Badge variant={taskStatusTone(step.status)}>{taskStatusLabel(step.status, $messages)}</Badge>
              </div>
              <div class="mt-2 text-sm text-muted-foreground">
                {formatTimestamp(step.startedAt)} {$messages.common.to} {formatTimestamp(step.finishedAt)}
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
      <CardHeader class="section-header">
        <CardTitle class="section-title" level="2">{$messages.tasks.taskLogs}</CardTitle>
        <div class="metric-label">{logStateLabel(logState)}</div>
      </CardHeader>
      <CardContent class="space-y-4">
        {#if logError}
          <Alert variant="destructive">
            <AlertTitle>{$messages.error.logStreamFailed}</AlertTitle>
            <AlertDescription>{logError}</AlertDescription>
            <Button type="button" variant="outline" size="sm" class="mt-3" onclick={retryLogStream}>
              {$messages.tasks.retryLogStream}
            </Button>
          </Alert>
        {/if}

        <!-- svelte-ignore a11y_no_noninteractive_tabindex -- Focusable read-only logs let keyboard users inspect streamed output. -->
        <pre
          class="task-log-accessible"
          role="log"
          tabindex="0"
          aria-label={$messages.tasks.accessibleLogLabel}
          aria-live={logLiveMode(logState, accessibleLogChunks.length > 0)}
          aria-relevant="additions text"
          aria-atomic="false"
        >{#if accessibleLogChunks.length}{#each accessibleLogChunks as chunk}{chunk}{/each}{:else}{$messages.tasks.waitingForOutput}{/if}</pre>

        <TerminalSurface
          active
          content={logContent}
          emptyText={$messages.tasks.waitingForOutput}
          heightClass="h-[360px] sm:h-[560px]"
        />
      </CardContent>
    </Card>
  </div>
</div>

<Dialog.Root bind:open={rollbackDialogOpen}>
  <Dialog.Content>
    <Dialog.Header>
      <Dialog.Title>{$messages.tasks.rollbackTitle}</Dialog.Title>
      <Dialog.Description>{$messages.tasks.rollbackDescription}</Dialog.Description>
    </Dialog.Header>
    <div class="space-y-3 py-2">
      <Label class="flex items-center gap-3 rounded-md border border-border/60 p-3 text-sm">
        <input type="checkbox" bind:checked={rollbackDns} disabled={!canRollbackDns()} />
        <span>{$messages.tasks.rollbackDns}</span>
      </Label>
      <Label class="flex items-center gap-3 rounded-md border border-border/60 p-3 text-sm">
        <input type="checkbox" bind:checked={deploySource} disabled={!canDeploySource()} />
        <span>{$messages.tasks.deploySource}</span>
      </Label>
      <Label class="flex items-center gap-3 rounded-md border border-border/60 p-3 text-sm">
        <input type="checkbox" bind:checked={stopTarget} disabled={!canStopTarget()} />
        <span>{$messages.tasks.stopTarget}</span>
      </Label>
      <Label class="flex items-center gap-3 rounded-md border border-border/60 p-3 text-sm text-muted-foreground">
        <input type="checkbox" disabled />
        <span>{$messages.tasks.cleanupTarget} · {$messages.tasks.cleanupUnavailable}</span>
      </Label>
    </div>
    <Dialog.Footer>
      <Button type="button" variant="outline" onclick={() => (rollbackDialogOpen = false)} disabled={creatingRollback}>
        {$messages.common.cancel}
      </Button>
      <Button type="button" onclick={createRollback} disabled={creatingRollback || (!rollbackDns && !deploySource && !stopTarget)}>
        {creatingRollback ? $messages.tasks.rollbackCreating : $messages.tasks.rollbackCreate}
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>
