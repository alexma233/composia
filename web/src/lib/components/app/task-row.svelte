<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import { TableCell, TableRow } from '$lib/components/ui/table';
  import { getMessages } from '$lib/i18n';

  const messages = getMessages();
  import { formatTimestamp, taskStatusLabel, taskStatusTone, taskTypeLabel } from '$lib/presenters';
  import type { TaskSummary } from '$lib/server/controller';
  interface Props {
    task: TaskSummary;
    showService?: boolean;
    showNode?: boolean;
  }

  let { task, showService = true, showNode = false }: Props = $props();
</script>

<TableRow class="hover:bg-accent/50">
  <TableCell>
    <a href={`/tasks/${task.taskId}`} class="font-medium hover:text-primary">
      {taskTypeLabel(task.type, $messages)}
      {#if showService && task.serviceName}
        <span class="text-muted-foreground">{$messages.common.for} {task.serviceName}</span>
      {:else if showNode && task.nodeId}
        <span class="text-muted-foreground">{$messages.common.on} {task.nodeId}</span>
      {/if}
    </a>
    <div class="text-xs text-muted-foreground">{task.taskId}</div>
  </TableCell>
  <TableCell>
    <Badge variant={taskStatusTone(task.status)}>{taskStatusLabel(task.status, $messages)}</Badge>
  </TableCell>
  <TableCell class="text-muted-foreground">{formatTimestamp(task.createdAt)}</TableCell>
</TableRow>
