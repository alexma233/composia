<script lang="ts">
  import { browser } from '$app/environment';
  import { onDestroy } from 'svelte';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { messages } from '$lib/i18n';

  interface Props {
    taskId?: string;
  }

  let { taskId = '' }: Props = $props();

  let content: string = $state('');
  let streamState: string = $state('idle');
  let errorMsg: string = $state('');
  let streamTaskId: string = $state('');
  let controller: AbortController | null = null;

  function streamStateLabel(state: string): string {
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

  $effect(() => {
    if (browser && taskId && taskId !== streamTaskId) {
      void startStream(taskId);
    }
  });

  $effect(() => {
    if (browser && !taskId && streamTaskId) {
      stopStream();
      content = '';
      streamState = 'idle';
      errorMsg = '';
      streamTaskId = '';
    }
  });

  onDestroy(stopStream);

  async function startStream(nextTaskId: string) {
    stopStream();
    controller = new AbortController();
    content = '';
    errorMsg = '';
    streamState = 'connecting';
    streamTaskId = nextTaskId;

    try {
      const response = await fetch(`/tasks/${nextTaskId}/logs`, {
        signal: controller.signal
      });
      if (!response.ok || !response.body) {
        throw new Error(`${$messages.error.logStreamFailed}: ${response.status}`);
      }

      streamState = 'streaming';
      const reader = response.body.getReader();
      const decoder = new TextDecoder();

      while (true) {
        const { done, value } = await reader.read();
        if (done) {
          break;
        }
        if (value) {
          content += decoder.decode(value, { stream: true });
        }
      }

      streamState = 'completed';
    } catch (err) {
      if (controller?.signal.aborted) {
        return;
      }
      streamState = 'failed';
      errorMsg = err instanceof Error ? err.message : $messages.error.logStreamFailed;
    }
  }

  function stopStream() {
    controller?.abort();
    controller = null;
  }
</script>

<div class="flex h-full min-h-0 flex-col">
  <div class="mb-3 flex items-center justify-between gap-3 text-xs font-medium text-muted-foreground">
    <span>{taskId ? $messages.tasks.logStreamTitle.replace('{taskId}', taskId) : $messages.tasks.noTaskSelected}</span>
    <span>{streamStateLabel(streamState)}</span>
  </div>

  {#if errorMsg}
    <Alert variant="destructive" class="mb-3">
      <AlertTitle>{$messages.error.logStreamFailed}</AlertTitle>
      <AlertDescription>{errorMsg}</AlertDescription>
    </Alert>
  {/if}

  <pre class="code-surface min-h-0 flex-1 overflow-auto" aria-live="polite" role="log">{content || $messages.tasks.logStreamEmpty}</pre>
</div>
