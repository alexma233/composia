<script lang="ts">
  import { browser } from '$app/environment';
  import { onDestroy } from 'svelte';

  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';

  interface Props {
    taskId?: string;
  }

  let { taskId = '' }: Props = $props();

  let content: string = $state('');
  let streamState: string = $state('idle');
  let errorMsg: string = $state('');
  let streamTaskId: string = $state('');
  let controller: AbortController | null = null;

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
        throw new Error(`Failed to tail task logs: ${response.status}`);
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
      errorMsg = err instanceof Error ? err.message : 'Failed to stream task logs.';
    }
  }

  function stopStream() {
    controller?.abort();
    controller = null;
  }
</script>

<div class="flex h-full min-h-0 flex-col">
  <div class="mb-3 flex items-center justify-between gap-3 text-xs font-medium text-muted-foreground">
    <span>{taskId ? `Task ${taskId}` : 'No task selected'}</span>
    <span>{streamState}</span>
  </div>

  {#if errorMsg}
    <Alert variant="destructive" class="mb-3">
      <AlertTitle>Log stream failed</AlertTitle>
      <AlertDescription>{errorMsg}</AlertDescription>
    </Alert>
  {/if}

  <pre class="min-h-0 flex-1 overflow-auto rounded-lg border border-border/70 bg-background/80 p-4 font-mono text-xs leading-6 whitespace-pre-wrap break-words">{content || 'Select a task to tail logs.'}</pre>
</div>
