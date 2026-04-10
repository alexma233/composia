<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import { messages } from '$lib/i18n';
  import { formatTimestamp, taskStatusLabel, taskStatusTone, taskTypeLabel } from '$lib/presenters';

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
  class="list-row"
>
  <div class="flex flex-wrap items-center justify-between gap-3">
    <div class="min-w-0 flex-1">
      <div class="truncate text-sm font-medium">
        {taskTypeLabel(task.type, $messages)}
        {#if showService && task.serviceName}
          <span class="text-muted-foreground">{$messages.common.for} {task.serviceName}</span>
        {:else if showNode && task.nodeId}
          <span class="text-muted-foreground">{$messages.common.on} {task.nodeId}</span>
        {/if}
      </div>
      <div class="truncate text-xs text-muted-foreground">{task.taskId}</div>
    </div>
    <Badge variant={taskStatusTone(task.status)}>{taskStatusLabel(task.status, $messages)}</Badge>
  </div>
  <div class="mt-2 text-xs text-muted-foreground">{formatTimestamp(task.createdAt)}</div>
</a>
