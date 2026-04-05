<script lang="ts">
  import { browser } from '$app/environment';
  import { onDestroy } from 'svelte';

  export let taskId = '';

  let content = '';
  let state = 'idle';
  let error = '';
  let streamTaskId = '';
  let controller: AbortController | null = null;

  $: if (browser && taskId && taskId !== streamTaskId) {
    void startStream(taskId);
  }

  $: if (browser && !taskId && streamTaskId) {
    stopStream();
    content = '';
    state = 'idle';
    error = '';
    streamTaskId = '';
  }

  onDestroy(stopStream);

  async function startStream(nextTaskId: string) {
    stopStream();
    controller = new AbortController();
    content = '';
    error = '';
    state = 'connecting';
    streamTaskId = nextTaskId;

    try {
      const response = await fetch(`/tasks/${nextTaskId}/logs`, {
        signal: controller.signal
      });
      if (!response.ok || !response.body) {
        throw new Error(`Failed to tail task logs: ${response.status}`);
      }

      state = 'streaming';
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

      state = 'completed';
    } catch (streamError) {
      if (controller?.signal.aborted) {
        return;
      }
      state = 'failed';
      error = streamError instanceof Error ? streamError.message : 'Failed to stream task logs.';
    }
  }

  function stopStream() {
    controller?.abort();
    controller = null;
  }
</script>

<div class="flex h-full min-h-0 flex-col">
  <div class="mb-3 flex items-center justify-between gap-3 text-xs uppercase tracking-[0.2em] text-muted-foreground">
    <span>{taskId ? `Task ${taskId}` : 'No task selected'}</span>
    <span>{state}</span>
  </div>

  {#if error}
    <div class="mb-3 rounded-lg border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive">
      {error}
    </div>
  {/if}

  <pre class="min-h-0 flex-1 overflow-auto rounded-lg border bg-background p-4 font-mono text-xs leading-6 whitespace-pre-wrap break-words">{content || 'Select a task to tail logs.'}</pre>
</div>
