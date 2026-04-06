<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import { formatTimestamp, taskStatusTone } from '$lib/presenters';

  interface TaskItem {
    taskId: string;
    type: string;
    status: string;
    serviceName?: string | null;
    nodeId?: string | null;
    createdAt: string;
  }

  interface Props {
    task: TaskItem;
    showService?: boolean;
    showNode?: boolean;
  }

  let { task, showService = true, showNode = false }: Props = $props();
</script>

<a
  href={`/tasks/${task.taskId}`}
  class="block rounded-lg border border-border/70 bg-background/80 px-4 py-3 transition-colors hover:bg-accent/60"
>
  <div class="flex flex-wrap items-center justify-between gap-3">
    <div class="min-w-0 flex-1">
      <div class="truncate text-sm font-medium">
        {task.type}
        {#if showService && task.serviceName}
          <span class="text-muted-foreground">for {task.serviceName}</span>
        {:else if showNode && task.nodeId}
          <span class="text-muted-foreground">on {task.nodeId}</span>
        {/if}
      </div>
      <div class="truncate text-xs text-muted-foreground">{task.taskId}</div>
    </div>
    <Badge variant={taskStatusTone(task.status)}>{task.status}</Badge>
  </div>
  <div class="mt-2 text-xs text-muted-foreground">{formatTimestamp(task.createdAt)}</div>
</a>